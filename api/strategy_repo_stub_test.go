package api

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type strategyRepoStub struct {
	repository.StrategyRepository
	get                   func(uuid.UUID) (*model.Strategy, error)
	getLatestPublishedRun func(uuid.UUID) (*model.StrategyRun, error)
	list                  func(repository.StrategyListFilter) ([]model.Strategy, error)
	addRun                func(model.StrategyRun) (*model.StrategyRun, error)
}

func (s strategyRepoStub) Get(id uuid.UUID) (*model.Strategy, error) {
	return s.get(id)
}

func (s strategyRepoStub) GetLatestPublishedRun(strategyID uuid.UUID) (*model.StrategyRun, error) {
	return s.getLatestPublishedRun(strategyID)
}

func (s strategyRepoStub) List(filter repository.StrategyListFilter) ([]model.Strategy, error) {
	return s.list(filter)
}

func (s strategyRepoStub) AddRun(m model.StrategyRun) (*model.StrategyRun, error) {
	return s.addRun(m)
}

func (s strategyRepoStub) Add(model.Strategy) (*model.Strategy, error) {
	panic("unexpected StrategyRepository.Add")
}

func (s strategyRepoStub) Update(model.Strategy, postgres.ColumnList) (*model.Strategy, error) {
	panic("unexpected StrategyRepository.Update")
}

func (s strategyRepoStub) GetIfBookmarked(model.Strategy) (*model.Strategy, error) {
	panic("unexpected StrategyRepository.GetIfBookmarked")
}
