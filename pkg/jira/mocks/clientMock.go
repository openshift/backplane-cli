// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/openshift/backplane-cli/pkg/jira (interfaces: JiraClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	jira "github.com/andygrunwald/go-jira"
	gomock "github.com/golang/mock/gomock"
)

// MockJiraClient is a mock of JiraClient interface.
type MockJiraClient struct {
	ctrl     *gomock.Controller
	recorder *MockJiraClientMockRecorder
}

// MockJiraClientMockRecorder is the mock recorder for MockJiraClient.
type MockJiraClientMockRecorder struct {
	mock *MockJiraClient
}

// NewMockJiraClient creates a new mock instance.
func NewMockJiraClient(ctrl *gomock.Controller) *MockJiraClient {
	mock := &MockJiraClient{ctrl: ctrl}
	mock.recorder = &MockJiraClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockJiraClient) EXPECT() *MockJiraClientMockRecorder {
	return m.recorder
}

// Connect mocks base method.
func (m *MockJiraClient) Connect(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Connect", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Connect indicates an expected call of Connect.
func (mr *MockJiraClientMockRecorder) Connect(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connect", reflect.TypeOf((*MockJiraClient)(nil).Connect), arg0, arg1)
}

// CreateIssue mocks base method.
func (m *MockJiraClient) CreateIssue(arg0 *jira.Issue) (*jira.Issue, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateIssue", arg0)
	ret0, _ := ret[0].(*jira.Issue)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateIssue indicates an expected call of CreateIssue.
func (mr *MockJiraClientMockRecorder) CreateIssue(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateIssue", reflect.TypeOf((*MockJiraClient)(nil).CreateIssue), arg0)
}

// SearchIssue mocks base method.
func (m *MockJiraClient) SearchIssue(arg0 string, arg1 *jira.SearchOptions) ([]jira.Issue, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SearchIssue", arg0, arg1)
	ret0, _ := ret[0].([]jira.Issue)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SearchIssue indicates an expected call of SearchIssue.
func (mr *MockJiraClientMockRecorder) SearchIssue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SearchIssue", reflect.TypeOf((*MockJiraClient)(nil).SearchIssue), arg0, arg1)
}