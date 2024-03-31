// Code generated by MockGen. DO NOT EDIT.
// Source: interest_rate.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
	model "factorbacktest/internal/db/models/postgres/public/model"
	domain "factorbacktest/internal/domain"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
)

// MockInterestRateRepository is a mock of InterestRateRepository interface.
type MockInterestRateRepository struct {
	ctrl     *gomock.Controller
	recorder *MockInterestRateRepositoryMockRecorder
}

// MockInterestRateRepositoryMockRecorder is the mock recorder for MockInterestRateRepository.
type MockInterestRateRepositoryMockRecorder struct {
	mock *MockInterestRateRepository
}

// NewMockInterestRateRepository creates a new mock instance.
func NewMockInterestRateRepository(ctrl *gomock.Controller) *MockInterestRateRepository {
	mock := &MockInterestRateRepository{ctrl: ctrl}
	mock.recorder = &MockInterestRateRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterestRateRepository) EXPECT() *MockInterestRateRepositoryMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m_2 *MockInterestRateRepository) Add(m model.InterestRate, tx *sql.Tx) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "Add", m, tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockInterestRateRepositoryMockRecorder) Add(m, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockInterestRateRepository)(nil).Add), m, tx)
}

// GetInterestRatesOnDates mocks base method.
func (m *MockInterestRateRepository) GetInterestRatesOnDates(arg0 []time.Time) ([]domain.InterestRateMap, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInterestRatesOnDates", arg0)
	ret0, _ := ret[0].([]domain.InterestRateMap)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInterestRatesOnDates indicates an expected call of GetInterestRatesOnDates.
func (mr *MockInterestRateRepositoryMockRecorder) GetInterestRatesOnDates(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInterestRatesOnDates", reflect.TypeOf((*MockInterestRateRepository)(nil).GetInterestRatesOnDates), arg0)
}

// GetRatesOnDay mocks base method.
func (m *MockInterestRateRepository) GetRatesOnDay(arg0 time.Time) (*domain.InterestRateMap, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRatesOnDay", arg0)
	ret0, _ := ret[0].(*domain.InterestRateMap)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRatesOnDay indicates an expected call of GetRatesOnDay.
func (mr *MockInterestRateRepositoryMockRecorder) GetRatesOnDay(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRatesOnDay", reflect.TypeOf((*MockInterestRateRepository)(nil).GetRatesOnDay), arg0)
}
