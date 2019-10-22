// Package asyncutil provides functionality for interacting with async operations
package async

import (
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/pkg/errors"
)

// Retry retries the given function until it doesn't fail. It doubles the
// period between attempts each time.
// Cribbed from https://upgear.io/blog/simple-golang-retry-function/
func Retry(attempts int, sleep time.Duration, fn func() error) error {
	start := time.Now()
	if err := innerRetry(attempts, sleep, fn); err != nil {
		end := time.Now()
		return pkgerrors.Wrapf(err,
			"failed after %d attempts and %s total duration",
			attempts, end.Sub(start))
	}
	return nil
}

func innerRetry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts > 1 {
			time.Sleep(sleep)
			return innerRetry(attempts-1, 2*sleep, fn)
		}
		return err
	}
	return nil
}

// RetryNoBackoff retries the given function until it doesn't fail. It keeps
// the amount of time between attempts constant.
// Cribbed from https://upgear.io/blog/simple-golang-retry-function/
func RetryNoBackoff(attempts int, sleep time.Duration, fn func() error) error {
	start := time.Now()
	if err := innerRetryNoBackoff(attempts, sleep, fn); err != nil {
		end := time.Now()
		return pkgerrors.Wrapf(err,
			"failed after %d attempts and %s total duration",
			attempts, end.Sub(start))
	}
	return nil
}

func innerRetryNoBackoff(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts > 1 {
			time.Sleep(sleep)
			return innerRetry(attempts-1, sleep, fn)
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
	start := time.Now()
	if !innerAwait(attempts, sleep, fn) {
		end := time.Now()
		msg := fmt.Sprintf("Condition was not true after %d attempts and %s total waiting time",
			attempts, end.Sub(start))
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
