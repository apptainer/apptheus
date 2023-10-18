// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/apptainer/apptheus/internal/cgroup/parser"
	"github.com/apptainer/apptheus/internal/monitor"
	"github.com/apptainer/apptheus/internal/storage"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/exporter-toolkit/web"
	"golang.org/x/sys/unix"
	"toolman.org/net/peercred"
)

type ServerOption struct {
	Server      *http.Server
	WebConfig   *web.FlagConfig
	MetricStore storage.MetricStore
	Logger      log.Logger
	SocketPath  string
	TrustedPath string
	Interval    *time.Ticker
	ErrCh       chan error
}

type WrappedInstance struct {
	*parser.ContainerInfo
	*monitor.Instance
	net.Conn
	Err error
}

type WrappedListener struct {
	*peercred.Listener
	TrustedPath string
	Option      *ServerOption
	ErrCh       chan *WrappedInstance
	DoneCh      chan *WrappedInstance
}

func (l *WrappedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	pid := conn.(*peercred.Conn).Ucred.Pid

	dirfd, err := unix.Open(fmt.Sprintf("/proc/%d", pid), unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}

	pidfd, err := unix.PidfdOpen(int(pid), 0)
	if err != nil {
		if !errors.Is(err, errors.ErrUnsupported) {
			return nil, err
		}
		level.Warn(l.Option.Logger).Log("alert", "host kernel does not support pidfd_open, silently ignored")
	}

	if err == nil {
		if err = unix.PidfdSendSignal(pidfd, 0, nil, 0); err != nil {
			return nil, err
		}
	}

	buf := make([]byte, 4096)
	n, err := unix.Readlinkat(dirfd, "exe", buf)
	if err != nil {
		return nil, err
	}
	link := string(buf[:n])

	exe := filepath.Base(link)
	verify := false
	for _, path := range strings.Split(l.TrustedPath, ";") {
		if strings.TrimSpace(link) == strings.TrimSpace(path) {
			verify = true
		}
	}

	if !verify {
		if conn != nil {
			conn.Close()
		}
		level.Error(l.Option.Logger).Log("msg", fmt.Sprintf("%s is not trusted, connection rejected", link))
		return conn, nil
	}

	// container and monitor instance info
	container := &parser.ContainerInfo{
		FullPath: link,
		Pid:      uint64(pid),
		Exe:      exe,
		ID:       fmt.Sprintf("%s_%d", exe, pid),
	}
	instance := monitor.New(l.Option.Interval)

	// save the container info for further usage
	wrappedInstance := &WrappedInstance{
		ContainerInfo: container,
		Instance:      instance,
		Conn:          conn,
	}

	// fire monitor thread
	go instance.Start(container, l.Option.MetricStore, l.Option.Logger)

	// fire a goroutine to retrieve the error or done message
	go func() {
		select {
		case err := <-instance.ErrCh:
			wrappedInstance.Err = err
			l.ErrCh <- wrappedInstance
			return
		case <-instance.Done:
			l.DoneCh <- wrappedInstance
			return
		}
	}()

	level.Info(l.Option.Logger).Log("msg", "New connection established", "container id", container.ID, "container pid", container.Pid, "container full path", container.FullPath)

	return conn, nil
}
