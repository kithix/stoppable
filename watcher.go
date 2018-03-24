package stoppable

import (
	"errors"
	"io"
	"sync"
)

// Errors
var (
	ErrAlreadyStarted = errors.New("Already started")
	ErrAlreadyStopped = errors.New("Already stopped")

	ErrSettingUp   = errors.New("Failure setting up")
	ErrDoing       = errors.New("Failure doing")
	ErrTearingDown = errors.New("Failure tearing down")
)

type Watcher struct {
	setup, do, teardown func() error
	restartHandler      func(WatcherError) bool
	closer              io.Closer
	mut                 sync.Mutex
}

func doWrapper(do func() error) (chan error, func() error) {
	interceptedStopping := make(chan error)
	return interceptedStopping, func() error {
		err := do()
		// If there's no error to stop on, don't bother signaling our teardown will close the channel.
		if err != nil {
			interceptedStopping <- err
		}
		// Proceed with typical error logic.
		return err
	}
}
func teardownWrapper(interceptedStopping chan error, teardown func() error) func() error {
	return func() error {
		err := teardown()
		// Close the channel to ensure the watcher moves on.
		close(interceptedStopping)
		return err
	}
}

func (w *Watcher) watcher(interceptedStopping chan error) {
	// Block here until we have an error to work with
	doingErr := <-interceptedStopping

	// We are in error handler mode, prevent being interacted with until we resolve.
	w.mut.Lock()
	defer w.mut.Unlock()

	// A nil error implies stopping was natural and we should not restart.
	if doingErr == nil {
		return
	}

	// We had an error during it running.
	watcherErr := WatcherError{
		Do: doingErr,
	}
	// Blocks until we retrieve the error value from teardown (if any)
	teardownErr := w.stop()
	if teardownErr != nil && teardownErr != doingErr {
		watcherErr.Teardown = teardownErr
	}
	// If we don't have a handler we can't try restart.
	// Handle current error state, if true we try start again.
	if w.restartHandler == nil || !w.restartHandler(watcherErr) {
		return
	}
	// Try start again
	setupErr := w.start()
	if setupErr != nil {
		// If we failed to start, add it to the stack
		watcherErr.Setup = setupErr
		// Send the setup error to our restartHandler.
		w.restartHandler(watcherErr)
	}
	// Watcher is now able to die.
}

func (w *Watcher) start() (err error) {
	if w.closer != nil {
		return ErrAlreadyStarted
	}
	// Wrap our functions
	interceptedStopping, wrappedDo := doWrapper(w.do)
	wrappedTeardown := teardownWrapper(interceptedStopping, w.teardown)
	w.closer, err = Open(w.setup, wrappedDo, wrappedTeardown)
	if err != nil {
		close(interceptedStopping)
		return err
	}
	go w.watcher(interceptedStopping)
	return nil
}

// Start will attempt to setup
func (w *Watcher) Start() (err error) {
	w.mut.Lock()
	defer w.mut.Unlock()
	return w.start()
}

func (w *Watcher) stop() error {
	if w.closer == nil {
		return ErrAlreadyStopped
	}
	defer func() { w.closer = nil }()
	return w.closer.Close()
}

func (w *Watcher) Stop() error {
	w.mut.Lock()
	defer w.mut.Unlock()
	return w.stop()
}

type WatcherError struct {
	Do       error
	Teardown error
	Setup    error
}

func (watcherErr *WatcherError) Error() string {
	stringBuilder := ""
	// If the watcher error has more than one error, print the do -> stop -> start stack.
	if watcherErr.Do != nil {
		stringBuilder += ErrDoing.Error() + ": " + watcherErr.Do.Error()
	}
	if watcherErr.Teardown != nil {
		if stringBuilder != "" {
			stringBuilder += " and "
		}
		stringBuilder += ErrTearingDown.Error() + ": " + watcherErr.Teardown.Error()
	}
	if watcherErr.Setup != nil {
		if stringBuilder != "" {
			stringBuilder += " and "
		}
		stringBuilder += ErrSettingUp.Error() + ": " + watcherErr.Setup.Error()
	}

	return ""
}

func RestartOnDo(watcherErr WatcherError) bool {
	if watcherErr.Teardown == nil && watcherErr.Setup == nil {
		return true
	}
	return false
}

func NewWatchdog(setup, do, teardown func() error, restartHandler func(WatcherError) bool) *Watcher {
	return &Watcher{
		setup:          setup,
		do:             do,
		teardown:       teardown,
		restartHandler: restartHandler,
	}
}
