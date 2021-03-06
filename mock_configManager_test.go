// Code generated by mockery v1.0.0. DO NOT EDIT.

package gocbcore

import mock "github.com/stretchr/testify/mock"

// mockConfigManager is an autogenerated mock type for the configManager type
type mockConfigManager struct {
	mock.Mock
}

// AddConfigWatcher provides a mock function with given fields: watcher
func (_m *mockConfigManager) AddConfigWatcher(watcher routeConfigWatcher) {
	_m.Called(watcher)
}

// RemoveConfigWatcher provides a mock function with given fields: watcher
func (_m *mockConfigManager) RemoveConfigWatcher(watcher routeConfigWatcher) {
	_m.Called(watcher)
}
