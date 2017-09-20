package fakes

import "time"

type Clock struct {
	SleepCall struct {
		CallCount int
		Receives  struct {
			Duration time.Duration
		}
	}
}

func (c *Clock) Sleep(duration time.Duration) {
	c.SleepCall.CallCount++
	c.SleepCall.Receives.Duration = duration
}
