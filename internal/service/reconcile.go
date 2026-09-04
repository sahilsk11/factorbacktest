package service

import (
	"context"
	"time"

	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
)

type ReconcileResult struct {
	Status    string       `json:"status"`
	CheckedAt time.Time    `json:"checkedAt"`
	Issues    []ReconIssue `json:"issues"`
}

type ReconIssue struct {
	Message      string  `json:"message"`
	InvestmentID *string `json:"investmentId,omitempty"`
}

func reconErrToIssue(err ReconErr) ReconIssue {
	issue := ReconIssue{Message: err.Message}
	if err.InvestmentID != nil {
		investmentID := err.InvestmentID.String()
		issue.InvestmentID = &investmentID
	}
	return issue
}

func (h investmentServiceHandler) RunReconcile(ctx context.Context) (*ReconcileResult, error) {
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	issues := make([]ReconIssue, 0)
	for _, investment := range investments {
		reconErrors, err := h.reconcileInvestment(ctx, investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		for _, reconErr := range reconErrors {
			issues = append(issues, reconErrToIssue(reconErr))
		}

		reconErr, err := h.reconcileTrades(investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		if reconErr != nil {
			issues = append(issues, reconErrToIssue(*reconErr))
		}
	}

	aggregateErrors, err := h.reconcileAggregatePortfolio()
	if err != nil {
		return nil, err
	}
	for _, reconErr := range aggregateErrors {
		issues = append(issues, reconErrToIssue(reconErr))
	}

	status := "OK"
	if len(issues) > 0 {
		status = "ISSUES"
	}

	return &ReconcileResult{
		Status:    status,
		CheckedAt: time.Now().UTC(),
		Issues:    issues,
	}, nil
}

func (h investmentServiceHandler) Reconcile(ctx context.Context) error {
	log := logger.FromContext(ctx)
	result, err := h.RunReconcile(ctx)
	if err != nil {
		return err
	}

	for _, issue := range result.Issues {
		if issue.InvestmentID != nil {
			log.Warnf("recon err on investment %s: %s", *issue.InvestmentID, issue.Message)
			continue
		}
		log.Warnf("recon err on aggregate portfolio: %s", issue.Message)
	}

	return nil
}
