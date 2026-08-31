// Package fingerprint derives a stable identifier for a normalized log
// template so repeated occurrences of the same structural event can be
// tracked in O(1) space regardless of message length.
package fingerprint

import (
	"hash/fnv"

	"github.com/InfraGuard-Labs/logquiet/internal/severity"
)

// ID is a 64-bit structural fingerprint. Collisions are theoretically
// possible (as with any hash) but at the pattern cardinalities a single
// terminal session will realistically see (thousands to low millions of
// distinct templates), the collision probability is negligible; see
// docs/TECHNICAL_METHOD.md.
type ID uint64

// Of computes the fingerprint of a normalized template combined with its
// severity level. Severity is included so that, for example, an ERROR and
// an INFO line that happen to normalize to the same text are tracked as
// distinct patterns rather than silently merged.
func Of(level severity.Level, template string) ID {
	h := fnv.New64a()
	_, _ = h.Write([]byte{byte(level)})
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(template))
	return ID(h.Sum64())
}
