// Code generated by MockGen. DO NOT EDIT.
// Source: internal/repository/interest_rate.repository.go

// Package mock_repository is a generated GoMock package.
package mock_repository

import (
	sql "database/sql"
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
func (m_2 *MockInterestRateRepository) Add(m domain.InterestRateMap, date time.Time, tx *sql.Tx) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "Add", m, date, tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockInterestRateRepositoryMockRecorder) Add(m, date, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockInterestRateRepository)(nil).Add), m, date, tx)
}

// GetRatesOnDate mocks base method.
func (m *MockInterestRateRepository) GetRatesOnDate(date time.Time, tx *sql.Tx) (*domain.InterestRateMap, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRatesOnDate", date, tx)
	ret0, _ := ret[0].(*domain.InterestRateMap)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRatesOnDate indicates an expected call of GetRatesOnDate.
func (mr *MockInterestRateRepositoryMockRecorder) GetRatesOnDate(date, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRatesOnDate", reflect.TypeOf((*MockInterestRateRepository)(nil).GetRatesOnDate), date, tx)
}

// GetRatesOnDates mocks base method.
func (m *MockInterestRateRepository) GetRatesOnDates(dates []time.Time, tx *sql.Tx) (map[string]domain.InterestRateMap, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRatesOnDates", dates, tx)
	ret0, _ := ret[0].(map[string]domain.InterestRateMap)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRatesOnDates indicates an expected call of GetRatesOnDates.
func (mr *MockInterestRateRepositoryMockRecorder) GetRatesOnDates(dates, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRatesOnDates", reflect.TypeOf((*MockInterestRateRepository)(nil).GetRatesOnDates), dates, tx)
}
