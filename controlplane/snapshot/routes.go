package snapshot

// TestRouteCount is the number of /test-{i} routes generated in each snapshot.
const TestRouteCount = 10000

// FilterChainCount is the number of matched filter chains on the ingress listener.
// Each chain carries a full HTTP filter stack to inflate LDS updates.
const FilterChainCount = 1000
