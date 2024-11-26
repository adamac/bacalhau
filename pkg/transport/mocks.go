// Code generated by MockGen. DO NOT EDIT.
// Source: types.go

// Package transport is a generated GoMock package.
package transport

import (
	reflect "reflect"

	envelope "github.com/bacalhau-project/bacalhau/pkg/lib/envelope"
	watcher "github.com/bacalhau-project/bacalhau/pkg/lib/watcher"
	gomock "go.uber.org/mock/gomock"
)

// MockMessageCreator is a mock of MessageCreator interface.
type MockMessageCreator struct {
	ctrl     *gomock.Controller
	recorder *MockMessageCreatorMockRecorder
}

// MockMessageCreatorMockRecorder is the mock recorder for MockMessageCreator.
type MockMessageCreatorMockRecorder struct {
	mock *MockMessageCreator
}

// NewMockMessageCreator creates a new mock instance.
func NewMockMessageCreator(ctrl *gomock.Controller) *MockMessageCreator {
	mock := &MockMessageCreator{ctrl: ctrl}
	mock.recorder = &MockMessageCreatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMessageCreator) EXPECT() *MockMessageCreatorMockRecorder {
	return m.recorder
}

// CreateMessage mocks base method.
func (m *MockMessageCreator) CreateMessage(event watcher.Event) (*envelope.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateMessage", event)
	ret0, _ := ret[0].(*envelope.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateMessage indicates an expected call of CreateMessage.
func (mr *MockMessageCreatorMockRecorder) CreateMessage(event interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateMessage", reflect.TypeOf((*MockMessageCreator)(nil).CreateMessage), event)
}
