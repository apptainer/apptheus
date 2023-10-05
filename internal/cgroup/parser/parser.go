// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

type Marshal interface {
	Marshal(*bytes.Buffer) (*bytes.Buffer, error)
}

type StatManager struct {
	funcs []StatFunc
	*cgroups.Stats
}

func (s *StatManager) add(fc StatFunc) {
	s.funcs = append(s.funcs, fc)
}

func (s *StatManager) WithCPU() *StatManager {
	s.add(func() (string, float64) {
		return "cpu_usage", float64(s.CpuStats.CpuUsage.TotalUsage)
	})
	return s
}

func (s *StatManager) WithMemory() *StatManager {
	s.add(func() (string, float64) {
		return "memory_usage", float64(s.MemoryStats.Usage.Usage)
	})
	return s
}

func (s *StatManager) WithMemorySwap() *StatManager {
	s.add(func() (string, float64) {
		return "memory_swap_usage", float64(s.MemoryStats.SwapUsage.Usage)
	})
	return s
}

func (s *StatManager) WithMemoryKernel() *StatManager {
	s.add(func() (string, float64) {
		return "memory_kernel_usage", float64(s.MemoryStats.KernelUsage.Usage)
	})
	return s
}

func (s *StatManager) WithPid() *StatManager {
	s.add(func() (string, float64) {
		return "pid_usage", float64(s.PidsStats.Current)
	})
	return s
}

func (s *StatManager) All() []StatFunc {
	return s.funcs
}

type StatFunc func() (string, float64)

type Stat interface {
	CreateStats() ([]StatFunc, error)
}

type ContainerInfo struct {
	FullPath string
	Pid      uint64
	Exe      string
	ID       string
}
