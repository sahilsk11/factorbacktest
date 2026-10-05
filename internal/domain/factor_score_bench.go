package domain

import "context"

type factorScoreDBContextKey struct{}

// WithFactorScoreDBDisabled marks ctx so factor score persistence and DB
// pre-reads are skipped (benchmark dry runs).
func WithFactorScoreDBDisabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, factorScoreDBContextKey{}, true)
}

// FactorScoreDBDisabled reports whether factor_score table read/write should
// be skipped for this request (env and/or context; see util.FactorScoreDBDisabled).
func FactorScoreDBDisabledFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(factorScoreDBContextKey{}).(bool)
	return ok && v
}
