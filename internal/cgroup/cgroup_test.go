package cgroup_test

import (
	"bytes"
	"testing"

	"github.com/apptainer/apptheus/internal/cgroup"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCgroupManager struct {
	mock.Mock
}

func (m *MockCgroupManager) Apply(_ int) error {
	return nil
}

func (m *MockCgroupManager) GetPids() ([]int, error) {
	return []int{}, nil
}

func (m *MockCgroupManager) GetAllPids() ([]int, error) {
	return []int{}, nil
}

func (m *MockCgroupManager) GetStats() (*cgroups.Stats, error) {
	return cgroups.NewStats(), nil
}

func (m *MockCgroupManager) Freeze(_ configs.FreezerState) error {
	return nil
}

func (m *MockCgroupManager) Destroy() error {
	return nil
}

func (m *MockCgroupManager) Path(path string) string {
	return path
}

func (m *MockCgroupManager) Set(_ *configs.Resources) error {
	return nil
}

func (m *MockCgroupManager) GetPaths() map[string]string {
	return map[string]string{}
}

func (m *MockCgroupManager) GetCgroups() (*configs.Cgroup, error) {
	return nil, nil
}

func (m *MockCgroupManager) GetFreezerState() (configs.FreezerState, error) {
	return "", nil
}

func (m *MockCgroupManager) Exists() bool {
	return true
}

func (m *MockCgroupManager) OOMKillCount() (uint64, error) {
	return 0, nil
}

func TestCgroup(t *testing.T) {
	cgroup := &cgroup.CGroup{
		Manager: &MockCgroupManager{},
	}

	has, err := cgroup.HasProcess()
	require.NoError(t, err)
	require.False(t, has)

	funcs, err := cgroup.CreateStats()
	require.NoError(t, err)
	require.NotEmpty(t, funcs)
	require.Len(t, funcs, 5)

	var buffer bytes.Buffer
	_, err = cgroup.Marshal(&buffer)
	require.NoError(t, err)
}
