package system

import "time"

/*
SystemClock is an outbound adapter for the Clock port.

The adapter sits outside the core and provides real wall-clock time
to the application service when the program runs normally.
*/
type SystemClock struct{}

/*
Now returns the current time in UTC.

Normalizing to UTC keeps persisted timestamps consistent.
*/
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
