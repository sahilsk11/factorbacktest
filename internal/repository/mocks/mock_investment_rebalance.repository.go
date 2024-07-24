// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/investment_rebalance.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	uuid "github.com/google/uuid"
)

// MockInvestmentRebalanceRepository is a mock of InvestmentRebalanceRepository interface.
type MockInvestmentRebalanceRepository struct {
	ctrl     *gomock.Controller
	recorder *MockInvestmentRebalanceRepositoryMockRecorder
}

// MockInvestmentRebalanceRepositoryMockRecorder is the mock recorder for MockInvestmentRebalanceRepository.
type MockInvestmentRebalanceRepositoryMockRecorder struct {
	mock *MockInvestmentRebalanceRepository
}

// NewMockInvestmentRebalanceRepository creates a new mock instance.
func NewMockInvestmentRebalanceRepository(ctrl *gomock.Controller) *MockInvestmentRebalanceRepository {
	mock := &MockInvestmentRebalanceRepository{ctrl: ctrl}
	mock.recorder = &MockInvestmentRebalanceRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInvestmentRebalanceRepository) EXPECT() *MockInvestmentRebalanceRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockInvestmentRebalanceRepository) Add(tx *sql.Tx, ir model.InvestmentRebalance) (*model.InvestmentRebalance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx, ir)
	ret0, _ := ret[0].(*model.InvestmentRebalance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockInvestmentRebalanceRepositoryMockRecorder) Add(tx, ir interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockInvestmentRebalanceRepository)(nil).Add), tx, ir)
}

// Get mocks base method.
func (m *MockInvestmentRebalanceRepository) Get(tx *sql.Tx, id uuid.UUID) (*model.InvestmentRebalance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", tx, id)
	ret0, _ := ret[0].(*model.InvestmentRebalance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockInvestmentRebalanceRepositoryMockRecorder) Get(tx, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockInvestmentRebalanceRepository)(nil).Get), tx, id)
}

// List mocks base method.
func (m *MockInvestmentRebalanceRepository) List(tx *sql.Tx) ([]model.InvestmentRebalance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", tx)
	ret0, _ := ret[0].([]model.InvestmentRebalance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockInvestmentRebalanceRepositoryMockRecorder) List(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockInvestmentRebalanceRepository)(nil).List), tx)
}
