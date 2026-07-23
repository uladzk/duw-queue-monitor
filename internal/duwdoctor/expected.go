package duwdoctor

import "github.com/uladzk/duw-queue-monitor/internal/queuemonitor"

// State is the queue-monitor state implied by either the DUW API (expected)
// or the channel's latest status post (observed).
type State string

const (
	StateInactive       State = "Inactive"
	StateActiveDisabled State = "ActiveDisabled"
	StateActiveEnabled  State = "ActiveEnabled"
)

// ExpectedFromQueue maps a DUW queue state to the state the monitor should reflect.
func ExpectedFromQueue(q queuemonitor.Queue) State {
	switch {
	case !q.Active:
		return StateInactive
	case !q.Enabled:
		return StateActiveDisabled
	default:
		return StateActiveEnabled
	}
}
