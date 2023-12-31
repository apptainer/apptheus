// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"math"
	"strings"
	"syscall"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

type Marshal interface {
	Marshal(buffer *bytes.Buffer) (*bytes.Buffer, error)
}

type StatManager struct {
	funcs []StatFunc
	*cgroups.Stats

	// for cpu metric
	prevTime, prevCPU uint64
}

func (s *StatManager) add(fc StatFunc) *StatManager {
	s.funcs = append(s.funcs, fc)
	return s
}

func (s *StatManager) WithCPU() *StatManager {
	return s.add(func() map[string]float64 {
		nowTime := time.Now()

		curTime := uint64(nowTime.UnixNano())
		curCPU := s.CpuStats.CpuUsage.TotalUsage

		deltaCPU := float64(curCPU - s.prevCPU)
		if s.prevTime == 0 {
			// 500ms earlier by default
			s.prevTime = uint64(nowTime.Truncate(500 * time.Millisecond).UnixNano())
		}

		deltaTime := float64(curTime - s.prevTime)
		cpuPercent := (deltaCPU / deltaTime) * 100

		// update the saved metrics
		s.prevTime = curTime
		s.prevCPU = curCPU
		return map[string]float64{
			"cpu_usage_per": cpuPercent,
			"cpu_prevTime":  float64(s.prevTime),
			"cpu_prevCpu":   float64(s.prevCPU),
		}
	})
}

func (s *StatManager) WithMemory() *StatManager {
	return s.add(func() map[string]float64 {
		memUsage := s.MemoryStats.Usage.Usage
		memLimit := s.MemoryStats.Usage.Limit
		memPercent := 0.0

		// If there is no limit, show system RAM instead of max uint64...
		if memLimit == math.MaxUint64 {
			in := &syscall.Sysinfo_t{}
			err := syscall.Sysinfo(in)
			if err == nil {
				memLimit = in.Totalram * uint64(in.Unit)
			}
		}
		if memLimit != 0 {
			memPercent = float64(memUsage) / float64(memLimit) * 100.0
		}
		return map[string]float64{
			"memory_usage_per": memPercent,
			"memory_usage":     float64(memUsage),
			"memory_limit":     float64(memLimit),
		}
	})
}

func (s *StatManager) WithMemorySwap() *StatManager {
	return s.add(func() map[string]float64 {
		swapUsage := s.MemoryStats.SwapUsage.Usage
		swapLimit := s.MemoryStats.SwapUsage.Limit
		swapPercent := 0.0

		// If there is no limit, show system RAM instead of max uint64...
		if swapLimit == math.MaxUint64 {
			in := &syscall.Sysinfo_t{}
			err := syscall.Sysinfo(in)
			if err == nil {
				swapLimit = in.Totalswap * uint64(in.Unit)
			}
		}
		if swapLimit != 0 {
			swapPercent = float64(swapUsage) / float64(swapLimit) * 100.0
		}
		return map[string]float64{
			"memory_swap_usage_per": swapPercent,
			"memory_swap":           float64(swapUsage),
			"memory_swap_limit":     float64(swapLimit),
		}
	})
}

func (s *StatManager) WithPid() *StatManager {
	return s.add(func() map[string]float64 {
		pidUsage := s.PidsStats.Current
		pidLimit := s.PidsStats.Limit
		pidPercent := 0.0

		if pidLimit != 0 {
			pidPercent = float64(pidUsage) / float64(pidLimit) * 100.0
		}
		return map[string]float64{
			"pid_usage_per": pidPercent,
			"pid_usage":     float64(pidUsage),
			"pid_limit":     float64(pidLimit),
		}
	})
}

func (s *StatManager) WithBlkIO() *StatManager {
	return s.add(func() map[string]float64 {
		var read, write float64
		for _, entry := range s.BlkioStats.IoQueuedRecursive {
			switch strings.ToLower(entry.Op) {
			case "read":
				read += float64(entry.Value)
			case "write":
				write += float64(entry.Value)
			}
		}
		return map[string]float64{
			"blkio_read":  read,
			"blkio_write": write,
		}
	})
}

func (s *StatManager) All() []StatFunc {
	return s.funcs
}

type StatFunc func() map[string]float64

type Stat interface {
	CreateStats() ([]StatFunc, error)
}

type ContainerInfo struct {
	FullPath string
	Pid      uint64
	Exe      string
	ID       string
}
