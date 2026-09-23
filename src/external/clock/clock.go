package clock

import "time"

// System reads the machine clock. It is an adapter because the time of day is
// something the outside world supplies, not something the agent can reason out.
type System struct{}

func NewSystem() System { return System{} }

func (System) Now() time.Time { return time.Now() }
