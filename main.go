// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"

	dto "github.com/prometheus/client_model/go"
	promlogflag "github.com/prometheus/common/promlog/flag"

	"github.com/apptainer/apptheus/internal/network"
	"github.com/apptainer/apptheus/internal/storage"
	"github.com/apptainer/apptheus/internal/util"
	"toolman.org/net/peercred"
)

const (
	VERSION = "0.1.0"
)

func init() {
	prometheus.MustRegister(version.NewCollector("apptheus"))
}

// logFunc in an adaptor to plug gokit logging into promhttp.HandlerOpts.
type logFunc func(...interface{}) error

func (lf logFunc) Println(v ...interface{}) {
	lf("msg", fmt.Sprintln(v...))
}

func main() {
	var (
		app                 = kingpin.New(filepath.Base(os.Args[0]), "The Apptheus")
		webConfig           = webflag.AddFlags(app, ":9091")
		metricsPath         = app.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		externalURL         = app.Flag("web.external-url", "The URL under which the Apptheus is externally reachable.").Default("").URL()
		routePrefix         = app.Flag("web.route-prefix", "Prefix for the internal routes of web endpoints. Defaults to the path of --web.external-url.").Default("").String()
		persistenceFile     = app.Flag("persistence.file", "File to persist metrics. If empty, metrics are only kept in memory.").Default("").String()
		persistenceInterval = app.Flag("persistence.interval", "The minimum interval at which to write out the persistence file.").Default("5m").Duration()
		promlogConfig       = promlog.Config{}
		socketPath          = app.Flag("socket.path", "Socket path for communication.").Default("/run/apptheus/gateway.sock").String()
		trustedPath         = app.Flag("trust.path", "Multiple trusted apptainer starter paths, use ';' to separate multiple entries").Default("").String()
		monitorInterval     = app.Flag("monitor.inverval", "The internval for sending system status.").Default("0.5s").Duration()
	)
	promlogflag.AddFlags(app, &promlogConfig)
	version.Version = VERSION
	app.Version(version.Print("apptheus"))
	app.HelpFlag.Short('h')
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger := promlog.New(&promlogConfig)

	*routePrefix = computeRoutePrefix(*routePrefix, *externalURL)
	level.Info(logger).Log("msg", "starting apptheus", "version", version.Info())

	// verify the caller is root or not
	isRoot, err := util.IsRoot()
	if err != nil {
		level.Error(logger).Log("msg", "Could not verify the caller", "err", err)
		os.Exit(-1)
	}

	if !isRoot {
		level.Info(logger).Log("msg", "Please launch using root user", "version", version.Info())
		os.Exit(-1)
	}

	// flags is used to show command line flags on the status page.
	// Kingpin default flags are excluded as they would be confusing.
	flags := map[string]string{}
	boilerplateFlags := kingpin.New("", "").Version("")
	for _, f := range app.Model().Flags {
		if boilerplateFlags.GetFlag(f.Name) == nil {
			flags[f.Name] = f.Value.String()
		}
	}

	ms := storage.NewDiskMetricStore(*persistenceFile, *persistenceInterval, prometheus.DefaultGatherer, logger)

	// Create a Gatherer combining the DefaultGatherer and the metrics from the metric store.
	g := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) { return ms.GetMetricFamilies(), nil }),
	}

	// server error channel
	errCh := make(chan error, 2)

	// verification server route
	verifyRoute := route.New()
	vmux := http.NewServeMux()
	vmux.Handle("/", decodeRequest(verifyRoute))
	verifyServer := &http.Server{Handler: vmux, ReadHeaderTimeout: time.Second}

	// create necessary parent folder for socket path
	parentFolder := path.Dir(*socketPath)
	if _, err := os.Stat(parentFolder); os.IsNotExist(err) {
		err := os.MkdirAll(parentFolder, 0o755)
		if err != nil {
			level.Error(logger).Log("msg", "Failed to create parent folder", "err", err)
		}
	}

	verificationOption := &network.ServerOption{
		Server:      verifyServer,
		WebConfig:   webConfig,
		MetricStore: ms,
		Logger:      logger,
		SocketPath:  *socketPath,
		TrustedPath: *trustedPath,
		Interval:    time.NewTicker(*monitorInterval),
		ErrCh:       errCh,
	}
	go startVerificationServer(verificationOption)

	// metrics server
	metricsRoute := route.New()
	mmux := http.NewServeMux()
	metricsRoute.Get(
		path.Join(*routePrefix, *metricsPath),
		promhttp.HandlerFor(g, promhttp.HandlerOpts{
			ErrorLog: logFunc(level.Error(logger).Log),
		}).ServeHTTP,
	)
	mmux.Handle("/", decodeRequest(metricsRoute))
	metricServer := &http.Server{Handler: mmux, ReadHeaderTimeout: time.Second}

	metricOption := &network.ServerOption{
		Server:      metricServer,
		WebConfig:   webConfig,
		MetricStore: ms,
		Logger:      logger,
		ErrCh:       errCh,
	}
	go startMetricsServer(metricOption)

	err = shutdownServerOnQuit(*socketPath, []*network.ServerOption{verificationOption, metricOption}, ms, errCh, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Failed to clean up the server", "err", err)
	}
}

func decodeRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close() // Make sure the underlying io.Reader is closed.
		switch contentEncoding := r.Header.Get("Content-Encoding"); strings.ToLower(contentEncoding) {
		case "gzip":
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = gr
		case "snappy":
			r.Body = io.NopCloser(snappy.NewReader(r.Body))
		default:
			// Do nothing.
		}

		h.ServeHTTP(w, r)
	})
}

// computeRoutePrefix returns the effective route prefix based on the
// provided flag values for --web.route-prefix and
// --web.external-url. With prefix empty, the path of externalURL is
// used instead. A prefix "/" results in an empty returned prefix. Any
// non-empty prefix is normalized to start, but not to end, with "/".
func computeRoutePrefix(prefix string, externalURL *url.URL) string {
	if prefix == "" {
		prefix = externalURL.Path
	}

	if prefix == "/" {
		prefix = ""
	}

	if prefix != "" {
		prefix = "/" + strings.Trim(prefix, "/")
	}

	return prefix
}

// shutdownServerOnQuit shutdowns the provided server upon closing the provided
// quitCh or upon receiving a SIGINT or SIGTERM.
func shutdownServerOnQuit(socketPath string, options []*network.ServerOption, ms *storage.DiskMetricStore, errCh <-chan error, logger log.Logger) error {
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, os.Interrupt, syscall.SIGTERM)

	select {
	case <-notifier:
		level.Info(logger).Log("msg", "received SIGINT/SIGTERM; exiting gracefully...")
		break
	case err := <-errCh:
		level.Warn(logger).Log("msg", "received error when launching server, exiting gracefully...", "err", err)
		break
	}

	defer os.Remove(socketPath)

	var retErr error
	for _, option := range options {
		err := option.Server.Shutdown(context.Background())
		if err != nil {
			level.Error(logger).Log("msg", "unable to shutdown the server", "err", err)
			retErr = errors.Join(retErr, err)
		}
	}

	err := ms.Shutdown()
	if err != nil {
		level.Error(logger).Log("msg", "unable to shutdown the storage service", "err", err)
		retErr = errors.Join(retErr, err)
	}
	return retErr
}

// startVerificationServer starts a verification server listening the unix socket
// it is also responsible for authentication via pid.
func startVerificationServer(option *network.ServerOption) {
	level.Info(option.Logger).Log("msg", "Start verification server")
	unixListener, err := peercred.Listen(context.Background(), option.SocketPath)
	if err != nil {
		level.Error(option.Logger).Log("msg", "Could not create local unix socket", "err", err)
		option.ErrCh <- err
		return
	}

	// chmod socketPath
	err = os.Chmod(option.SocketPath, 0o777)
	if err != nil {
		level.Error(option.Logger).Log("msg", "Could not chmod local unix socket", "err", err)
		option.ErrCh <- err
		return
	}

	listener := network.WrappedListener{
		Listener:    unixListener,
		TrustedPath: option.TrustedPath,
		Option:      option,
		ErrCh:       make(chan *network.WrappedInstance, 1),
		DoneCh:      make(chan *network.WrappedInstance, 1),
	}

	quitCh := make(chan struct{}, 1)

	go func() {
		err = web.Serve(&listener, option.Server, option.WebConfig, option.Logger)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				level.Info(option.Logger).Log("msg", "Verification server stopped")
			} else {
				level.Error(option.Logger).Log("msg", "Verification server stopped with error", "err", err)
				option.ErrCh <- err
			}
		}

		quitCh <- struct{}{}
	}()

	for {
		select {
		case <-quitCh:
			// stop all loop
			return
		case wrappedInstance := <-listener.ErrCh:
			level.Error(option.Logger).Log("msg", "Container monitor instance receieved error", "container id", wrappedInstance.ContainerInfo.ID, "err", wrappedInstance.Err)
			// server side closes the connection in case client side misses (in theory client will close the connection first)
			if wrappedInstance.Conn != nil {
				wrappedInstance.Conn.Close()
			}
		case wrappedInstance := <-listener.DoneCh:
			level.Info(option.Logger).Log("msg", "Container monitor instance completed, will close the connection", "container id", wrappedInstance.ContainerInfo.ID)
			// server side closes the connection in case client side misses (in theory client will close the connection first)
			if wrappedInstance.Conn != nil {
				wrappedInstance.Conn.Close()
			}
		}
	}
}

// startMetricsServer starts the `/metrics` endpoints, exposing metrics
func startMetricsServer(option *network.ServerOption) {
	level.Info(option.Logger).Log("msg", "Start metrics server")
	err := web.ListenAndServe(option.Server, option.WebConfig, option.Logger)
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			level.Info(option.Logger).Log("msg", "Metrics server stopped")
		} else {
			level.Error(option.Logger).Log("msg", "Metrics server stopped", "err", err)
			option.ErrCh <- err
		}
	}
}
