package parser_test

import (
	"testing"

	"github.com/apptainer/apptheus/internal/cgroup/parser"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	stats := cgroups.NewStats()
	mgr := &parser.StatManager{
		Stats: stats,
	}

	require.NotNil(t, mgr)
	allFuncs := mgr.WithCPU().WithMemory().WithMemorySwap().WithPid().WithBlkIO().All()
	require.Len(t, allFuncs, 5)

	usage := allFuncs[0]()
	require.InEpsilon(t, 0.01, usage["cpu_usage_per"], 1)

	usage = allFuncs[1]()
	require.InEpsilon(t, 0.01, usage["memory_usage_per"], 1)

	usage = allFuncs[2]()
	require.InEpsilon(t, 0.01, usage["memory_swap_usage_per"], 1)

	usage = allFuncs[3]()
	require.InEpsilon(t, 0.01, usage["pid_usage_per"], 1)

	usage = allFuncs[4]()
	require.InEpsilon(t, 0.01, usage["blkio_read"], 1)
	require.InEpsilon(t, 0.01, usage["blkio_write"], 1)
}
