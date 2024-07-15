// Code generated by MockGen. DO NOT EDIT.
// Source: internal/service/l2/factor_expression.service.go

// Package mock_l2_service is a generated GoMock package.
package mock_l2_service

import (
	context "context"
	model "factorbacktest/internal/db/models/postgres/public/model"
	l1_service "factorbacktest/internal/service/l1"
	l2_service "factorbacktest/internal/service/l2"
	reflect "reflect"
	time "time"

	qrm "github.com/go-jet/jet/v2/qrm"
	gomock "github.com/golang/mock/gomock"
)

// MockFactorExpressionService is a mock of FactorExpressionService interface.
type MockFactorExpressionService struct {
	ctrl     *gomock.Controller
	recorder *MockFactorExpressionServiceMockRecorder
}

// MockFactorExpressionServiceMockRecorder is the mock recorder for MockFactorExpressionService.
type MockFactorExpressionServiceMockRecorder struct {
	mock *MockFactorExpressionService
}

// NewMockFactorExpressionService creates a new mock instance.
func NewMockFactorExpressionService(ctrl *gomock.Controller) *MockFactorExpressionService {
	mock := &MockFactorExpressionService{ctrl: ctrl}
	mock.recorder = &MockFactorExpressionServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFactorExpressionService) EXPECT() *MockFactorExpressionServiceMockRecorder {
	return m.recorder
}

// CalculateFactorScores mocks base method.
func (m *MockFactorExpressionService) CalculateFactorScores(ctx context.Context, tradingDays []time.Time, tickers []model.Ticker, factorExpression string) (map[time.Time]*l2_service.ScoresResultsOnDay, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CalculateFactorScores", ctx, tradingDays, tickers, factorExpression)
	ret0, _ := ret[0].(map[time.Time]*l2_service.ScoresResultsOnDay)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CalculateFactorScores indicates an expected call of CalculateFactorScores.
func (mr *MockFactorExpressionServiceMockRecorder) CalculateFactorScores(ctx, tradingDays, tickers, factorExpression interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalculateFactorScores", reflect.TypeOf((*MockFactorExpressionService)(nil).CalculateFactorScores), ctx, tradingDays, tickers, factorExpression)
}

// CalculateFactorScoresOnDay mocks base method.
func (m *MockFactorExpressionService) CalculateFactorScoresOnDay(ctx context.Context, date time.Time, tickers []model.Ticker, factorExpression string) (*l2_service.ScoresResultsOnDay, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CalculateFactorScoresOnDay", ctx, date, tickers, factorExpression)
	ret0, _ := ret[0].(*l2_service.ScoresResultsOnDay)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CalculateFactorScoresOnDay indicates an expected call of CalculateFactorScoresOnDay.
func (mr *MockFactorExpressionServiceMockRecorder) CalculateFactorScoresOnDay(ctx, date, tickers, factorExpression interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalculateFactorScoresOnDay", reflect.TypeOf((*MockFactorExpressionService)(nil).CalculateFactorScoresOnDay), ctx, date, tickers, factorExpression)
}

// MockfactorMetricCalculations is a mock of factorMetricCalculations interface.
type MockfactorMetricCalculations struct {
	ctrl     *gomock.Controller
	recorder *MockfactorMetricCalculationsMockRecorder
}

// MockfactorMetricCalculationsMockRecorder is the mock recorder for MockfactorMetricCalculations.
type MockfactorMetricCalculationsMockRecorder struct {
	mock *MockfactorMetricCalculations
}

// NewMockfactorMetricCalculations creates a new mock instance.
func NewMockfactorMetricCalculations(ctrl *gomock.Controller) *MockfactorMetricCalculations {
	mock := &MockfactorMetricCalculations{ctrl: ctrl}
	mock.recorder = &MockfactorMetricCalculationsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockfactorMetricCalculations) EXPECT() *MockfactorMetricCalculationsMockRecorder {
	return m.recorder
}

// AnnualizedStdevOfDailyReturns mocks base method.
func (m *MockfactorMetricCalculations) AnnualizedStdevOfDailyReturns(ctx context.Context, pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AnnualizedStdevOfDailyReturns", ctx, pr, symbol, start, end)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AnnualizedStdevOfDailyReturns indicates an expected call of AnnualizedStdevOfDailyReturns.
func (mr *MockfactorMetricCalculationsMockRecorder) AnnualizedStdevOfDailyReturns(ctx, pr, symbol, start, end interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AnnualizedStdevOfDailyReturns", reflect.TypeOf((*MockfactorMetricCalculations)(nil).AnnualizedStdevOfDailyReturns), ctx, pr, symbol, start, end)
}

// MarketCap mocks base method.
func (m *MockfactorMetricCalculations) MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarketCap", tx, symbol, date)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MarketCap indicates an expected call of MarketCap.
func (mr *MockfactorMetricCalculationsMockRecorder) MarketCap(tx, symbol, date interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarketCap", reflect.TypeOf((*MockfactorMetricCalculations)(nil).MarketCap), tx, symbol, date)
}

// PbRatio mocks base method.
func (m *MockfactorMetricCalculations) PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PbRatio", tx, symbol, date)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PbRatio indicates an expected call of PbRatio.
func (mr *MockfactorMetricCalculationsMockRecorder) PbRatio(tx, symbol, date interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PbRatio", reflect.TypeOf((*MockfactorMetricCalculations)(nil).PbRatio), tx, symbol, date)
}

// PeRatio mocks base method.
func (m *MockfactorMetricCalculations) PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeRatio", tx, symbol, date)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PeRatio indicates an expected call of PeRatio.
func (mr *MockfactorMetricCalculationsMockRecorder) PeRatio(tx, symbol, date interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeRatio", reflect.TypeOf((*MockfactorMetricCalculations)(nil).PeRatio), tx, symbol, date)
}

// Price mocks base method.
func (m *MockfactorMetricCalculations) Price(pr *l1_service.PriceCache, symbol string, date time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Price", pr, symbol, date)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Price indicates an expected call of Price.
func (mr *MockfactorMetricCalculationsMockRecorder) Price(pr, symbol, date interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Price", reflect.TypeOf((*MockfactorMetricCalculations)(nil).Price), pr, symbol, date)
}

// PricePercentChange mocks base method.
func (m *MockfactorMetricCalculations) PricePercentChange(pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PricePercentChange", pr, symbol, start, end)
	ret0, _ := ret[0].(float64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PricePercentChange indicates an expected call of PricePercentChange.
func (mr *MockfactorMetricCalculationsMockRecorder) PricePercentChange(pr, symbol, start, end interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PricePercentChange", reflect.TypeOf((*MockfactorMetricCalculations)(nil).PricePercentChange), pr, symbol, start, end)
}