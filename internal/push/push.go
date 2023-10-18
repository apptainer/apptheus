// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package push

import (
	"bytes"
	"errors"
	"time"

	"github.com/apptainer/apptheus/internal/storage"
	"github.com/prometheus/common/expfmt"
)

func Push(ms storage.MetricStore, data []byte, labels map[string]string) error {
	if _, ok := labels["job"]; !ok {
		return errors.New("job should be set in labels")
	}

	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	ms.SubmitWriteRequest(storage.WriteRequest{
		Labels:         labels,
		Timestamp:      time.Now(),
		MetricFamilies: metricFamilies,
		Replace:        false,
		Done:           errCh,
	})

	for err := range errCh {
		return err
	}
	return nil
}
