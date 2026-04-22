package testseed

import (
	"fmt"

	"github.com/google/uuid"
)

// lookupUUID fetches a uuid.UUID value from a dependency's Result.
func lookupUUID(deps map[string]Result, fixture, key string) (uuid.UUID, error) {
	res, ok := deps[fixture]
	if !ok {
		return uuid.Nil, fmt.Errorf("missing dependency %q", fixture)
	}
	raw, ok := res[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("dependency %q missing key %q", fixture, key)
	}
	id, ok := raw.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("dependency %q key %q is %T, want uuid.UUID", fixture, key, raw)
	}
	return id, nil
}
