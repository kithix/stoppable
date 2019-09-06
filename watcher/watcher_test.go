package stoppable

import (
	"errors"
	"testing"
	"time"

	"github.com/kithix/stoppable"
)

// Errors used for testing
var (
	ErrTest = errors.New("Test Error")
)

func TestWatchdog(t *testing.T) {
	setup := false
	hasDone := 0
	done := make(chan struct{})
	teardown := false

	watchdog := New(
		func() error {
			setup = true
			return nil
		},
		func() error {
			select {
			case done <- struct{}{}:
			case <-time.After(1 * time.Nanosecond):
			}
			hasDone++
			return nil
		},
		func() error {
			teardown = true
			return nil
		},
		nil)
	err := watchdog.Start()
	if err != nil {
		t.Error(err)
	}
	err = watchdog.Start()
	if err != ErrAlreadyStarted {
		t.Error(err)
	}

	<-done
	err = watchdog.Stop()
	if setup == false {
		t.Error("Did not run setup")
	}
	// Unsure how to guarantee inner do loop only runs once.
	if hasDone == 0 {
		t.Error("Did not do anything")
	}
	if teardown == false {
		t.Error("Did not run teardown")
	}
	if err != nil {
		t.Error(err)
	}

	err = watchdog.Stop()
	if err != ErrAlreadyStopped {
		t.Error(err)
	}
}

func TestWatcherSetupFail(t *testing.T) {
	watchdog := New(
		func() error { return ErrTest },
		stoppable.DoesNothing,
		stoppable.DoesNothing,
		nil,
	)

	err := watchdog.Start()
	if err != ErrTest {
		t.Error(err)
	}
	// Ensure it tries to start the error again
	err = watchdog.Start()
	if err != ErrTest {
		t.Error(err)
	}
	// Ensure it is stopped.
	err = watchdog.Stop()
	if err != ErrAlreadyStopped {
		t.Error(err)
	}
}

func TestWatcherDoFails(t *testing.T) {
	awaiting := make(chan struct{})
	watchdog := New(stoppable.DoesNothing, func() error {
		close(awaiting)
		return ErrTest
	}, stoppable.DoesNothing, nil)

	if err := watchdog.Start(); err != nil {
		t.Error(err)
	}
	<-awaiting

	if err := watchdog.Stop(); err != ErrAlreadyStopped && err != ErrTest {
		t.Error(err)
	}
}

func TestWatchdogAutomaticRestart(t *testing.T) {
	awaiting := make(chan struct{})
	watchdog := New(stoppable.DoesNothing, func() error {
		<-awaiting
		return ErrTest
	}, stoppable.DoesNothing, RestartOnDo)

	if err := watchdog.Start(); err != nil {
		t.Error(err)
	}
	close(awaiting)
	// We could catch our watchdog at any state (setup, do, teardown) when initiating the stop.
	// We cannot however receive an 'ErrAlreadyStopped' unlike above
	if err := watchdog.Stop(); err != ErrTest && err != nil {
		t.Error(err)
	}
}
