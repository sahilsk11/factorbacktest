// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/alpaca.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	repository "factorbacktest/internal/repository"
	reflect "reflect"

	alpaca "github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	gomock "github.com/golang/mock/gomock"
	uuid "github.com/google/uuid"
)

// MockAlpacaRepository is a mock of AlpacaRepository interface.
type MockAlpacaRepository struct {
	ctrl     *gomock.Controller
	recorder *MockAlpacaRepositoryMockRecorder
}

// MockAlpacaRepositoryMockRecorder is the mock recorder for MockAlpacaRepository.
type MockAlpacaRepositoryMockRecorder struct {
	mock *MockAlpacaRepository
}

// NewMockAlpacaRepository creates a new mock instance.
func NewMockAlpacaRepository(ctrl *gomock.Controller) *MockAlpacaRepository {
	mock := &MockAlpacaRepository{ctrl: ctrl}
	mock.recorder = &MockAlpacaRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAlpacaRepository) EXPECT() *MockAlpacaRepositoryMockRecorder {
	return m.recorder
}

// CancelOpenOrders mocks base method.
func (m *MockAlpacaRepository) CancelOpenOrders() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelOpenOrders")
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelOpenOrders indicates an expected call of CancelOpenOrders.
func (mr *MockAlpacaRepositoryMockRecorder) CancelOpenOrders() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelOpenOrders", reflect.TypeOf((*MockAlpacaRepository)(nil).CancelOpenOrders))
}

// GetAccount mocks base method.
func (m *MockAlpacaRepository) GetAccount() (*alpaca.Account, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccount")
	ret0, _ := ret[0].(*alpaca.Account)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAccount indicates an expected call of GetAccount.
func (mr *MockAlpacaRepositoryMockRecorder) GetAccount() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccount", reflect.TypeOf((*MockAlpacaRepository)(nil).GetAccount))
}

// GetOrder mocks base method.
func (m *MockAlpacaRepository) GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOrder", alpacaOrderID)
	ret0, _ := ret[0].(*alpaca.Order)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOrder indicates an expected call of GetOrder.
func (mr *MockAlpacaRepositoryMockRecorder) GetOrder(alpacaOrderID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOrder", reflect.TypeOf((*MockAlpacaRepository)(nil).GetOrder), alpacaOrderID)
}

// GetPositions mocks base method.
func (m *MockAlpacaRepository) GetPositions() ([]alpaca.Position, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPositions")
	ret0, _ := ret[0].([]alpaca.Position)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPositions indicates an expected call of GetPositions.
func (mr *MockAlpacaRepositoryMockRecorder) GetPositions() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPositions", reflect.TypeOf((*MockAlpacaRepository)(nil).GetPositions))
}

// IsMarketOpen mocks base method.
func (m *MockAlpacaRepository) IsMarketOpen() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsMarketOpen")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsMarketOpen indicates an expected call of IsMarketOpen.
func (mr *MockAlpacaRepositoryMockRecorder) IsMarketOpen() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsMarketOpen", reflect.TypeOf((*MockAlpacaRepository)(nil).IsMarketOpen))
}

// PlaceOrder mocks base method.
func (m *MockAlpacaRepository) PlaceOrder(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PlaceOrder", req)
	ret0, _ := ret[0].(*alpaca.Order)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PlaceOrder indicates an expected call of PlaceOrder.
func (mr *MockAlpacaRepositoryMockRecorder) PlaceOrder(req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PlaceOrder", reflect.TypeOf((*MockAlpacaRepository)(nil).PlaceOrder), req)
}
