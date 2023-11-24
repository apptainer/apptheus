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

func (m *MockCgroupManager) Apply(pid int) error {
	args := m.Called(pid)
	return args.Error(0)
}

func (m *MockCgroupManager) GetPids() ([]int, error) {
	args := m.Called()
	return args.Get(0).([]int), args.Error(1)
}

func (m *MockCgroupManager) GetAllPids() ([]int, error) {
	args := m.Called()
	return args.Get(0).([]int), args.Error(1)
}

func (m *MockCgroupManager) GetStats() (*cgroups.Stats, error) {
	args := m.Called()
	return args.Get(0).(*cgroups.Stats), args.Error(1)
}

func (m *MockCgroupManager) Freeze(state configs.FreezerState) error {
	args := m.Called(state)
	return args.Error(0)
}

func (m *MockCgroupManager) Destroy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCgroupManager) Path(path string) string {
	args := m.Called(path)
	return args.String(0)
}

func (m *MockCgroupManager) Set(resource *configs.Resources) error {
	args := m.Called(resource)
	return args.Error(0)
}

func (m *MockCgroupManager) GetPaths() map[string]string {
	args := m.Called()
	return args.Get(0).(map[string]string)
}

func (m *MockCgroupManager) GetCgroups() (*configs.Cgroup, error) {
	args := m.Called()
	return args.Get(0).(*configs.Cgroup), args.Error(1)
}

func (m *MockCgroupManager) GetFreezerState() (configs.FreezerState, error) {
	args := m.Called()
	return args.Get(0).(configs.FreezerState), args.Error(1)
}

func (m *MockCgroupManager) Exists() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockCgroupManager) OOMKillCount() (uint64, error) {
	args := m.Called()
	return uint64(args.Int(0)), args.Error(1)
}

func TestCgroup(t *testing.T) {
	mockManager := new(MockCgroupManager)
	// set mock behaviors
	mockManager.On("Apply", mock.Anything).Return(nil)
	mockManager.On("GetPids").Return([]int{}, nil)
	mockManager.On("GetAllPids").Return([]int{}, nil)
	mockManager.On("GetStats").Return(cgroups.NewStats(), nil)
	mockManager.On("Freeze", mock.Anything).Return(nil)
	mockManager.On("Destroy").Return(nil)
	mockManager.On("Path", mock.Anything).Return("")
	mockManager.On("Set", mock.Anything).Return(nil)
	mockManager.On("GetPaths").Return(map[string]string{})
	mockManager.On("GetCgroups").Return(nil, nil)
	mockManager.On("GetFreezerState").Return("", nil)
	mockManager.On("Exists").Return(false)
	mockManager.On("OOMKillCount").Return(0, nil)

	cgroup := &cgroup.CGroup{
		Manager: mockManager,
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
