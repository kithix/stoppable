package stoppable

import (
	"testing"
	"time"
)

func TestWatchdog(t *testing.T) {
	setup := false
	hasDone := 0
	done := make(chan struct{})
	teardown := false

	watchdog := NewWatchdog(
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
		})
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

func TestWatchdogSetupFail(t *testing.T) {
	watchdog := NewWatchdog(func() error { return TestErr }, DoesNothing, DoesNothing)

	err := watchdog.Start()
	if err != TestErr {
		t.Error(err)
	}
	// Ensure it tries to start the error again
	err = watchdog.Start()
	if err != TestErr {
		t.Error(err)
	}
	// Ensure it is stopped.
	err = watchdog.Stop()
	if err != ErrAlreadyStopped {
		t.Error(err)
	}
}

func TestWatchdogDoFails(t *testing.T) {
	awaiting := make(chan struct{})
	watchdog := NewWatchdog(DoesNothing, func() error {
		close(awaiting)
		return TestErr
	}, DoesNothing)

	if err := watchdog.Start(); err != nil {
		t.Error(err)
	}
	<-awaiting

	if err := watchdog.Stop(); err != ErrAlreadyStopped && err != TestErr {
		t.Error(err)
	}
}

func TestWatchdogAutomaticRestart(t *testing.T) {
	awaiting := make(chan struct{})
	watchdog := NewWatchdog(DoesNothing, func() error {
		<-awaiting
		return TestErr
	}, DoesNothing)
	watchdog.restartHandler = RestartOnDo

	if err := watchdog.Start(); err != nil {
		t.Error(err)
	}
	close(awaiting)
	// We could catch our watchdog at any state (setup, do, teardown) when initiating the stop.
	// We cannot however receive an 'ErrAlreadyStopped' unlike above
	if err := watchdog.Stop(); err != TestErr && err != nil {
		t.Error(err)
	}
}
