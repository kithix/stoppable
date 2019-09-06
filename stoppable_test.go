package stoppable

import (
	"errors"
	"testing"
	"time"
)

// Test Errors
var (
	ErrTest = errors.New("Test Error")
)

func TestOpenAndClose(t *testing.T) {
	setup := false
	hasDone := 0
	done := make(chan struct{})
	teardown := false

	stoppable, err := Open(
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
	if err != nil {
		t.Error(err)
	}
	<-done
	err = stoppable.Close()
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

	err = stoppable.Close()
	if err != ErrAlreadyClosed {
		t.Error(err)
	}
}

func TestSetupFails(t *testing.T) {
	_, err := Open(
		func() error {
			return ErrTest
		}, DoesNothing, DoesNothing)

	if err != ErrTest {
		t.Error(err)
	}
}

func TestDoingFails(t *testing.T) {
	// To ensure our goroutine inside runs.
	doingHasRun := make(chan struct{})

	stoppable, err := Open(
		DoesNothing, func() error {
			// We close, if this runs again it will panic.
			close(doingHasRun)
			return ErrTest
		}, DoesNothing,
	)
	if err != nil {
		t.Error(err)
	}
	// Let it run.
	<-doingHasRun

	err = stoppable.Close()
	if err != ErrTest {
		t.Error(err)
	}
}

func TestTeardownFails(t *testing.T) {
	stoppable, err := Open(DoesNothing, DoesNothing, func() error {
		return ErrTest
	})
	if err != nil {
		t.Error(err)
	}
	err = stoppable.Close()
	if err != ErrTest {
		t.Error(err)
	}
}

func TestDoingAndTeardownFails(t *testing.T) {
	// To ensure our goroutine inside runs.
	doingHasRun := make(chan struct{})

	stoppable, err := Open(DoesNothing, func() error {
		// We close, if this runs again it will panic.
		close(doingHasRun)
		return errors.New("DoingError")
	}, func() error {
		return errors.New("TeardownError")
	})

	if err != nil {
		t.Error(err)
	}
	<-doingHasRun

	err = stoppable.Close()
	if err.Error() != "DoingError"+"TeardownError" {
		t.Error(err)
	}
}
