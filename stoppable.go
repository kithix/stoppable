package stoppable

import (
	"errors"
	"sync"
	"time"
)

// Errors for stoppable lifecycle problems
var (
	ErrAlreadyClosed = errors.New("Already closed")
)

// DoesNothing is a function that returns instantly with no error, used for matching the function signature of func() error
var DoesNothing = func() error { return nil }

// Task is a long running function that can signal when it errors and can be signaled to stop.
type Task func(stopper chan struct{}, stopping chan error)

// Closer is a convenience container for the channels sent to a task.
type Closer struct {
	sync.Mutex // Used to prevent it being closed twice.
	closed     bool

	stopper  chan struct{}
	stopping chan error
	timeout  time.Duration
}

// Close sends a stop signal to the running task and blocks until an error type returns.
// If a task has stopped before close it will be blocked waiting until close is called.
func (s *Closer) Close() (err error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return ErrAlreadyClosed
	}
	defer func() { s.closed = true }()

	// Signals stop by closing the stopper.
	close(s.stopper)

	// Return the error back from the running task
	return <-s.stopping
}

func newTask(setup, do, teardown func() error) Task {
	return func(stopper chan struct{}, stopping chan error) {
		var err error
		var doing = true

		defer close(stopping)

		err = setup()
		if err != nil {
			stopping <- err
			return
		}
		for doing {
			select {
			// Do until error
			default:
				if err = do(); err != nil {
					// Exit loop
					doing = false
				}
			case <-stopper:
				doing = false
			}
		}

		tearDownErr := teardown()
		if tearDownErr != nil {
			if err != nil {
				err = errors.New(err.Error() + tearDownErr.Error())
			} else {
				err = tearDownErr
			}
		}

		stopping <- err
		return
	}
}

// Open creates a stoppable goroutine with the three lifecycle functions provided.
// There is no safety, a user can create a deadlock themselves here.
func Open(setup, do, teardown func() error) (*Closer, error) {
	err := setup()
	if err != nil {
		return nil, err
	}

	stoppableFunc := newTask(DoesNothing, do, teardown)
	stopper := make(chan struct{})
	stopping := make(chan error)

	go stoppableFunc(stopper, stopping)

	return &Closer{
		stopper:  stopper,
		stopping: stopping,
	}, nil
}
