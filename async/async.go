// Package asyncutil provides functionality for interacting with async operations
package async

import (
	"errors"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
)


// WaitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
// Cribbed from https://stackoverflow.com/a/32843750/10359642
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

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
			return innerRetryNoBackoff(attempts-1, sleep, fn)
		}
		return err
	}
	return nil
}

// AwaitNoBackoff attempts the given condition the specified amount of times,
// sleeping between each attempt. If the condition doesn't succeed,
// it returns an error saying how many times we tried and how much time it
// took altogether.
func AwaitNoBackoff(attempts int, sleep time.Duration, fn func() bool, msgs ...string) error {
	return await(attempts, sleep, fn, innerAwaitNoBackoff, msgs...)
}

// Await attempts the given condition the specified amount of times, doubling
// the amount of time between each attempt. If the condition doesn't succeed,
// it returns an error saying how many times we tried and how much time it
// took altogether.
func Await(attempts int, sleep time.Duration, fn func() bool, msgs ...string) error {
	return await(attempts, sleep, fn, innerAwait, msgs...)
}

func await(attempts int, sleep time.Duration, fn func() bool, waiter func(int, time.Duration, func() bool) bool, msgs ...string) error {
	start := time.Now()
	if !waiter(attempts, sleep, fn) {
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

func innerAwaitNoBackoff(attempts int, sleep time.Duration, fn func() bool) bool {
	if !fn() {
		if attempts > 1 {
			time.Sleep(sleep)
			return innerAwaitNoBackoff(attempts-1, 2, fn)
		}
		return false
	}
	return true
}

func innerAwait(attempts int, sleep time.Duration, fn func() bool) bool {
	if !fn() {
		if attempts > 1 {
			time.Sleep(sleep)
			return innerAwait(attempts-1, 2*sleep, fn)
		}
		return false
	}
	return true
}
