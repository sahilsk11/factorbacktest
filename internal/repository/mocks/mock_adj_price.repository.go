// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/adj_price.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	domain "factorbacktest/internal/domain"
	repository "factorbacktest/internal/repository"
	reflect "reflect"
	time "time"

	qrm "github.com/go-jet/jet/v2/qrm"
	gomock "github.com/golang/mock/gomock"
)

// MockAdjustedPriceRepository is a mock of AdjustedPriceRepository interface.
type MockAdjustedPriceRepository struct {
	ctrl     *gomock.Controller
	recorder *MockAdjustedPriceRepositoryMockRecorder
}

// MockAdjustedPriceRepositoryMockRecorder is the mock recorder for MockAdjustedPriceRepository.
type MockAdjustedPriceRepositoryMockRecorder struct {
	mock *MockAdjustedPriceRepository
}

// NewMockAdjustedPriceRepository creates a new mock instance.
func NewMockAdjustedPriceRepository(ctrl *gomock.Controller) *MockAdjustedPriceRepository {
	mock := &MockAdjustedPriceRepository{ctrl: ctrl}
	mock.recorder = &MockAdjustedPriceRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAdjustedPriceRepository) EXPECT() *MockAdjustedPriceRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockAdjustedPriceRepository) Add(arg0 *sql.Tx, arg1 []model.AdjustedPrice) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockAdjustedPriceRepositoryMockRecorder) Add(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).Add), arg0, arg1)
}

// Get mocks base method.
func (m *MockAdjustedPriceRepository) Get(arg0 *sql.Tx, arg1 string, arg2 time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockAdjustedPriceRepositoryMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).Get), arg0, arg1, arg2)
}

// GetMany mocks base method.
func (m *MockAdjustedPriceRepository) GetMany(arg0 *sql.Tx, arg1 []string, arg2 time.Time) (map[string]float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMany", arg0, arg1, arg2)
	ret0, _ := ret[0].(map[string]float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMany indicates an expected call of GetMany.
func (mr *MockAdjustedPriceRepositoryMockRecorder) GetMany(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMany", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).GetMany), arg0, arg1, arg2)
}

// LatestPrices mocks base method.
func (m *MockAdjustedPriceRepository) LatestPrices(tx *sql.Tx, symbols []string) ([]domain.AssetPrice, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LatestPrices", tx, symbols)
	ret0, _ := ret[0].([]domain.AssetPrice)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LatestPrices indicates an expected call of LatestPrices.
func (mr *MockAdjustedPriceRepositoryMockRecorder) LatestPrices(tx, symbols interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestPrices", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).LatestPrices), tx, symbols)
}

// List mocks base method.
func (m *MockAdjustedPriceRepository) List(db qrm.Queryable, symbols []string, start, end time.Time) ([]domain.AssetPrice, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", db, symbols, start, end)
	ret0, _ := ret[0].([]domain.AssetPrice)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockAdjustedPriceRepositoryMockRecorder) List(db, symbols, start, end interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).List), db, symbols, start, end)
}

// ListFromSet mocks base method.
func (m *MockAdjustedPriceRepository) ListFromSet(tx *sql.Tx, set []repository.ListFromSetInput) ([]domain.AssetPrice, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListFromSet", tx, set)
	ret0, _ := ret[0].([]domain.AssetPrice)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListFromSet indicates an expected call of ListFromSet.
func (mr *MockAdjustedPriceRepositoryMockRecorder) ListFromSet(tx, set interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListFromSet", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).ListFromSet), tx, set)
}

// ListTradingDays mocks base method.
func (m *MockAdjustedPriceRepository) ListTradingDays(tx *sql.Tx, start, end time.Time) ([]time.Time, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTradingDays", tx, start, end)
	ret0, _ := ret[0].([]time.Time)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTradingDays indicates an expected call of ListTradingDays.
func (mr *MockAdjustedPriceRepositoryMockRecorder) ListTradingDays(tx, start, end interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTradingDays", reflect.TypeOf((*MockAdjustedPriceRepository)(nil).ListTradingDays), tx, start, end)
}
