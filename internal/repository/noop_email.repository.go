package repository

// NewNoopEmailRepository returns an EmailRepository that silently
// discards every send. Use only in test/dev environments where we
// boot the API without real email credentials (see cmd/test-api).
// It is selected via EMAIL_PROVIDER=noop in cmd/util.go and is NOT
// reachable through the normal prod config paths.
func NewNoopEmailRepository() EmailRepository {
	return noopEmailRepositoryHandler{}
}

type noopEmailRepositoryHandler struct{}

func (noopEmailRepositoryHandler) SendEmail(_, _, _ string) error { return nil }
