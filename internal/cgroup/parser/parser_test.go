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
	require.Equal(t, 0.0, usage["cpu_usage"])

	usage = allFuncs[1]()
	require.Equal(t, 0.0, usage["memory_usage"])

	usage = allFuncs[2]()
	require.Equal(t, 0.0, usage["memory_swap_usage"])

	usage = allFuncs[3]()
	require.Equal(t, 0.0, usage["pid_usage"])

	usage = allFuncs[4]()
	require.Equal(t, 0.0, usage["blkio_read"])
	require.Equal(t, 0.0, usage["blkio_write"])
}
