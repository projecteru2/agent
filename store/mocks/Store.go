// Code generated by mockery v1.0.0
package mocks

import mock "github.com/stretchr/testify/mock"

import types "github.com/projecteru2/agent/types"

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// GetAllContainers provides a mock function with given fields:
func (_m *Store) GetAllContainers() ([]string, error) {
	ret := _m.Called()

	var r0 []string
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetContainer provides a mock function with given fields: cid
func (_m *Store) GetContainer(cid string) (*types.Container, error) {
	ret := _m.Called(cid)

	var r0 *types.Container
	if rf, ok := ret.Get(0).(func(string) *types.Container); ok {
		r0 = rf(cid)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(cid)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RegisterNode provides a mock function with given fields: node
func (_m *Store) RegisterNode(node *types.Node) error {
	ret := _m.Called(node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.Node) error); ok {
		r0 = rf(node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveContainer provides a mock function with given fields: cid
func (_m *Store) RemoveContainer(cid string) error {
	ret := _m.Called(cid)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(cid)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateContainer provides a mock function with given fields: container
func (_m *Store) UpdateContainer(container *types.Container) error {
	ret := _m.Called(container)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.Container) error); ok {
		r0 = rf(container)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateStats provides a mock function with given fields: node
func (_m *Store) UpdateStats(node *types.Node) error {
	ret := _m.Called(node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.Node) error); ok {
		r0 = rf(node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
