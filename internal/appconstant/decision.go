package appconstant

const (
	PriorityMust   = "must"
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"
)

// PriorityWeights maps a watchlist priority to its Priority-Based decision
// weight. A candidate absent from session_priority_snapshots (not on a
// participant's active watchlist at session creation) gets weight 0 — that
// case is handled by the resolver's zero-initialized totals, not listed
// here.
var PriorityWeights = map[string]int{
	PriorityMust:   5,
	PriorityHigh:   3,
	PriorityMedium: 2,
	PriorityLow:    1,
}
