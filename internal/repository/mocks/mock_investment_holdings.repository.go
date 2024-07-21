// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/investment_holdings.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	domain "factorbacktest/internal/domain"
	repository "factorbacktest/internal/repository"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	uuid "github.com/google/uuid"
)

// MockInvestmentHoldingsRepository is a mock of InvestmentHoldingsRepository interface.
type MockInvestmentHoldingsRepository struct {
	ctrl     *gomock.Controller
	recorder *MockInvestmentHoldingsRepositoryMockRecorder
}

// MockInvestmentHoldingsRepositoryMockRecorder is the mock recorder for MockInvestmentHoldingsRepository.
type MockInvestmentHoldingsRepositoryMockRecorder struct {
	mock *MockInvestmentHoldingsRepository
}

// NewMockInvestmentHoldingsRepository creates a new mock instance.
func NewMockInvestmentHoldingsRepository(ctrl *gomock.Controller) *MockInvestmentHoldingsRepository {
	mock := &MockInvestmentHoldingsRepository{ctrl: ctrl}
	mock.recorder = &MockInvestmentHoldingsRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInvestmentHoldingsRepository) EXPECT() *MockInvestmentHoldingsRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockInvestmentHoldingsRepository) Add(tx *sql.Tx, sih model.InvestmentHoldings) (*model.InvestmentHoldings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx, sih)
	ret0, _ := ret[0].(*model.InvestmentHoldings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockInvestmentHoldingsRepositoryMockRecorder) Add(tx, sih interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockInvestmentHoldingsRepository)(nil).Add), tx, sih)
}

// Get mocks base method.
func (m *MockInvestmentHoldingsRepository) Get(id uuid.UUID) (*model.InvestmentHoldings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", id)
	ret0, _ := ret[0].(*model.InvestmentHoldings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockInvestmentHoldingsRepositoryMockRecorder) Get(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockInvestmentHoldingsRepository)(nil).Get), id)
}

// GetLatestHoldings mocks base method.
func (m *MockInvestmentHoldingsRepository) GetLatestHoldings(tx *sql.Tx, investmentID uuid.UUID) (*domain.Portfolio, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestHoldings", tx, investmentID)
	ret0, _ := ret[0].(*domain.Portfolio)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestHoldings indicates an expected call of GetLatestHoldings.
func (mr *MockInvestmentHoldingsRepositoryMockRecorder) GetLatestHoldings(tx, investmentID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestHoldings", reflect.TypeOf((*MockInvestmentHoldingsRepository)(nil).GetLatestHoldings), tx, investmentID)
}

// List mocks base method.
func (m *MockInvestmentHoldingsRepository) List(arg0 repository.HoldingsListFilter) ([]model.InvestmentHoldings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].([]model.InvestmentHoldings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockInvestmentHoldingsRepositoryMockRecorder) List(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockInvestmentHoldingsRepository)(nil).List), arg0)
}
