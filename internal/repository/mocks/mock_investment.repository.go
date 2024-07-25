// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/investment.repository.go
//
// Generated by this command:
//
//	mockgen -source=internal/repository/investment.repository.go -destination=internal/repository/mocks/mock_investment.repository.go
//

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	repository "factorbacktest/internal/repository"
	reflect "reflect"

	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockInvestmentRepository is a mock of InvestmentRepository interface.
type MockInvestmentRepository struct {
	ctrl     *gomock.Controller
	recorder *MockInvestmentRepositoryMockRecorder
}

// MockInvestmentRepositoryMockRecorder is the mock recorder for MockInvestmentRepository.
type MockInvestmentRepositoryMockRecorder struct {
	mock *MockInvestmentRepository
}

// NewMockInvestmentRepository creates a new mock instance.
func NewMockInvestmentRepository(ctrl *gomock.Controller) *MockInvestmentRepository {
	mock := &MockInvestmentRepository{ctrl: ctrl}
	mock.recorder = &MockInvestmentRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInvestmentRepository) EXPECT() *MockInvestmentRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockInvestmentRepository) Add(tx *sql.Tx, si model.Investment) (*model.Investment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx, si)
	ret0, _ := ret[0].(*model.Investment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockInvestmentRepositoryMockRecorder) Add(tx, si any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockInvestmentRepository)(nil).Add), tx, si)
}

// Get mocks base method.
func (m *MockInvestmentRepository) Get(id uuid.UUID) (*model.Investment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", id)
	ret0, _ := ret[0].(*model.Investment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockInvestmentRepositoryMockRecorder) Get(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockInvestmentRepository)(nil).Get), id)
}

// List mocks base method.
func (m *MockInvestmentRepository) List(arg0 repository.StrategyInvestmentListFilter) ([]model.Investment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].([]model.Investment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockInvestmentRepositoryMockRecorder) List(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockInvestmentRepository)(nil).List), arg0)
}
