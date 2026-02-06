package fetcher

import (
	"container/list"
)

// Export constants for testing
const (
	MaxDrainBytes        = maxDrainBytes
	DefaultMaxBytes      = defaultMaxBytes
	MinRetries           = minAllowedRetries
	MaxAllowedRetries    = maxAllowedRetries
	DefaultMaxRetryDelay = defaultMaxRetryDelay
)

// Export variables for testing
// Note: These expose the variables themselves, but since they are unexported in the package,
// we can only expose their values or provide functions to modify them if they are vars.
// For slices/maps, exporting the value allows modification of the underlying data.
var (
	// DefaultUserAgents exposes the default UA list.
	// Since it's a slice, modifications to elements will affect the package.
	DefaultUserAgents = defaultUserAgents

	// DefaultTransport exposes the internal default transport.
	DefaultTransport = defaultTransport
)

// Export functions for testing
var DrainAndCloseBody = drainAndCloseBody
var NormalizeByteLimit = normalizeByteLimit

// Helper functions for white-box testing

// ResetTransportCache resets the internal transport cache.
// This is crucial for verifying cache behavior without interference between tests.
func ResetTransportCache() {
	transportCacheMu.Lock()
	defer transportCacheMu.Unlock()
	transportCache = make(map[transportCacheKey]*list.Element)
	transportCacheLRU.Init()
}

// SetDefaultUserAgents allows overwriting the default UA list for deterministic testing.
func SetDefaultUserAgents(uas []string) {
	defaultUserAgents = uas
}
