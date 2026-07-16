package appconstant

const (
	PriorityMust   = "must"
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"
)

// Decision session statuses — mirrors the decision_sessions.status check
// constraint. A session is always created straight into Voting; Open exists
// in the DB constraint but is never produced by this API.
const (
	SessionStatusOpen      = "open"
	SessionStatusVoting    = "voting"
	SessionStatusCompleted = "completed"
	SessionStatusCancelled = "cancelled"
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
