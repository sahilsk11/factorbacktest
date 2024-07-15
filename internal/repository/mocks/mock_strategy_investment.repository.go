// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/strategy_investment.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	repository "factorbacktest/internal/repository"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	uuid "github.com/google/uuid"
)

// MockStrategyInvestmentRepository is a mock of StrategyInvestmentRepository interface.
type MockStrategyInvestmentRepository struct {
	ctrl     *gomock.Controller
	recorder *MockStrategyInvestmentRepositoryMockRecorder
}

// MockStrategyInvestmentRepositoryMockRecorder is the mock recorder for MockStrategyInvestmentRepository.
type MockStrategyInvestmentRepositoryMockRecorder struct {
	mock *MockStrategyInvestmentRepository
}

// NewMockStrategyInvestmentRepository creates a new mock instance.
func NewMockStrategyInvestmentRepository(ctrl *gomock.Controller) *MockStrategyInvestmentRepository {
	mock := &MockStrategyInvestmentRepository{ctrl: ctrl}
	mock.recorder = &MockStrategyInvestmentRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStrategyInvestmentRepository) EXPECT() *MockStrategyInvestmentRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockStrategyInvestmentRepository) Add(tx *sql.Tx, si model.StrategyInvestment) (*model.StrategyInvestment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx, si)
	ret0, _ := ret[0].(*model.StrategyInvestment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockStrategyInvestmentRepositoryMockRecorder) Add(tx, si interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockStrategyInvestmentRepository)(nil).Add), tx, si)
}

// Get mocks base method.
func (m *MockStrategyInvestmentRepository) Get(id uuid.UUID) (*model.StrategyInvestment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", id)
	ret0, _ := ret[0].(*model.StrategyInvestment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockStrategyInvestmentRepositoryMockRecorder) Get(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockStrategyInvestmentRepository)(nil).Get), id)
}

// List mocks base method.
func (m *MockStrategyInvestmentRepository) List(arg0 repository.StrategyInvestmentListFilter) ([]model.StrategyInvestment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].([]model.StrategyInvestment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockStrategyInvestmentRepositoryMockRecorder) List(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockStrategyInvestmentRepository)(nil).List), arg0)
}