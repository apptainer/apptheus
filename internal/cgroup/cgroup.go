// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package cgroup

import (
	"bytes"
	"fmt"

	"github.com/apptainer/apptheus/internal/cgroup/parser"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/manager"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const gateway = "metric_gateway"

type CGroup struct {
	cgroups.Manager
}

func NewCGroup(path string) (*CGroup, error) {
	cg := &configs.Cgroup{Resources: &configs.Resources{}}
	cg.Path = fmt.Sprintf("/%s/%s", gateway, path)
	mgr, err := manager.New(cg)
	if err != nil {
		return nil, err
	}
	return &CGroup{Manager: mgr}, nil
}

func (c *CGroup) HasProcess() (bool, error) {
	pids, err := c.GetPids()
	return len(pids) != 0, err
}

func (c *CGroup) CreateStats() ([]parser.StatFunc, error) {
	stat, err := c.Manager.GetStats()
	if err != nil {
		return nil, err
	}

	statManager := &parser.StatManager{Stats: stat}
	statManager.WithCPU().WithMemory().WithMemorySwap().WithMemoryKernel().WithPid()
	return statManager.All(), nil
}

func (c *CGroup) Marshal(buffer *bytes.Buffer) (*bytes.Buffer, error) {
	stats, err := c.CreateStats()
	if err != nil {
		return nil, err
	}

	// write stats
	for _, stat := range stats {
		key, val := stat()
		fmt.Fprintf(buffer, "%s %f\n", key, val)
	}

	return buffer, nil
}
