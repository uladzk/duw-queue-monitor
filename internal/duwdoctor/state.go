package duwdoctor

import (
	"encoding/json"
	"strings"
	"time"
)

// DoctorState is the debounce/cooldown state persisted between checker runs.
type DoctorState struct {
	ConsecutiveMismatches int       `json:"consecutiveMismatches"`
	LastEscalationAt      time.Time `json:"lastEscalationAt"`
	LastReason            string    `json:"lastReason"`
}

// ParseState decodes the ConfigMap-stored state. An empty payload is the zero state.
func ParseState(data string) (DoctorState, error) {
	if strings.TrimSpace(data) == "" {
		return DoctorState{}, nil
	}
	var s DoctorState
	err := json.Unmarshal([]byte(data), &s)
	return s, err
}

// Marshal encodes the state for ConfigMap storage.
func (s DoctorState) Marshal() (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

// InCooldown reports whether an escalation fired within the cooldown window before now.
func (s DoctorState) InCooldown(now time.Time, cooldown time.Duration) bool {
	if s.LastEscalationAt.IsZero() {
		return false
	}
	return now.Before(s.LastEscalationAt.Add(cooldown))
}
