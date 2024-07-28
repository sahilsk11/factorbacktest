// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/trade_order.repository.go
//
// Generated by this command:
//
//	mockgen -source=internal/repository/trade_order.repository.go -destination=internal/repository/mocks/mock_trade_order.repository.go
//

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	repository "factorbacktest/internal/repository"
	reflect "reflect"

	postgres "github.com/go-jet/jet/v2/postgres"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockTradeOrderRepository is a mock of TradeOrderRepository interface.
type MockTradeOrderRepository struct {
	ctrl     *gomock.Controller
	recorder *MockTradeOrderRepositoryMockRecorder
}

// MockTradeOrderRepositoryMockRecorder is the mock recorder for MockTradeOrderRepository.
type MockTradeOrderRepositoryMockRecorder struct {
	mock *MockTradeOrderRepository
}

// NewMockTradeOrderRepository creates a new mock instance.
func NewMockTradeOrderRepository(ctrl *gomock.Controller) *MockTradeOrderRepository {
	mock := &MockTradeOrderRepository{ctrl: ctrl}
	mock.recorder = &MockTradeOrderRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTradeOrderRepository) EXPECT() *MockTradeOrderRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockTradeOrderRepository) Add(tx *sql.Tx, to model.TradeOrder) (*model.TradeOrder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", tx, to)
	ret0, _ := ret[0].(*model.TradeOrder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockTradeOrderRepositoryMockRecorder) Add(tx, to any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockTradeOrderRepository)(nil).Add), tx, to)
}

// Get mocks base method.
func (m *MockTradeOrderRepository) Get(arg0 repository.TradeOrderGetFilter) (*model.TradeOrder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(*model.TradeOrder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockTradeOrderRepositoryMockRecorder) Get(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockTradeOrderRepository)(nil).Get), arg0)
}

// List mocks base method.
func (m *MockTradeOrderRepository) List() ([]model.TradeOrder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]model.TradeOrder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockTradeOrderRepositoryMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockTradeOrderRepository)(nil).List))
}

// Update mocks base method.
func (m *MockTradeOrderRepository) Update(tx *sql.Tx, tradeOrderID uuid.UUID, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", tx, tradeOrderID, to, columns)
	ret0, _ := ret[0].(*model.TradeOrder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockTradeOrderRepositoryMockRecorder) Update(tx, tradeOrderID, to, columns any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockTradeOrderRepository)(nil).Update), tx, tradeOrderID, to, columns)
}
