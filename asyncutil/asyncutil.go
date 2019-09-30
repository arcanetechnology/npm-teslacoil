// Package asyncutil provides functionality for interacting with async operations
package asyncutil

import (
	"errors"
	"fmt"
	"time"
)

// Retry retries the given function until it doesn't fail. It doubles the
// period between attempts each time.
// Cribbed from https://upgear.io/blog/simple-golang-retry-function/
func Retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts > 1 {
			time.Sleep(sleep)
			return Retry(attempts-1, 2*sleep, fn)
		}
		return err
	}
	return nil
}

// Await attempts the given condition the specified amount of times, doubling
// the amount of time between each attempt. If the condition doesn't succeed,
// it returns an error saying how many times we tried and how much time it
// took altogether.
func Await(attempts int, sleep time.Duration, fn func() bool, msgs ...string) error {
	if !innerAwait(attempts, sleep, fn) {
		msg := fmt.Sprintf("Condition was not true after %d attempts and %s total waiting time",
			attempts, GetTotalRetryDuration(attempts, sleep))
		if len(msgs) != 0 {
			msg += ": "
			for _, m := range msgs {
				msg += m + " "
			}
		}
		return errors.New(msg)
	}
	return nil
}

func innerAwait(attempts int, sleep time.Duration,
	fn func() bool) bool {
	if !fn() {
		if attempts > 1 {
			time.Sleep(sleep)
			return innerAwait(attempts-1, 2*sleep, fn)
		}
		return false
	}
	return true
}

// GetTotalRetryDuration calculates the total amount of time spent retrying
// an operation given the amount of attempts and initial sleep duration
func GetTotalRetryDuration(attempts int, sleep time.Duration) time.Duration {
	if attempts <= 0 {
		return sleep
	}
	return sleep + GetTotalRetryDuration(attempts-1, sleep*2)
}
