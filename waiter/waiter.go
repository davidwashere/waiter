package waiter

import (
	"context"
	"errors"
)

var (
	ErrWaiterClosed = errors.New("waiter has been closed")
)

// Waiter waits for a value to be published to it, typically via another goroutine
//
// The Manager that built this waiter is responsible for passing the
// expected value
//
// It is the callers responsibility to ensure that either Wait finishes (via value or ctx.Done)
// or Close is invoked so that proper cleanup can be done
type Waiter[T any] interface {
	// ID contains the unique ID assigned to this waiter by the Managers ID generator
	ID() string

	// Wait will block for a value, context expiration, or closed waiter (whichever occurs first)
	// error will be non-nil on context expiration or closed waiter
	Wait(ctx context.Context) (T, error)

	// Close signals the waiter to and its manager
	// to cleanup and no longer process responses
	// for this waiter
	Close()
}

type genericWaiter[T any] struct {
	// the unique ID generated for this waiter
	id string

	// the chan for receiveing a msg
	ch chan *response[T]

	// the managers chan used to signal that this waiter is done
	managerDoneCh chan string

	// when closed indicates this waiter should stop / cleanup
	doneCh chan struct{}
}

var _ Waiter[struct{}] = (*genericWaiter[struct{}])(nil)

func newGenericWaiter[T any](id string, ch chan *response[T], managerDoneCh chan string) *genericWaiter[T] {
	return &genericWaiter[T]{
		id:            id,
		ch:            ch,
		managerDoneCh: managerDoneCh,
		doneCh:        make(chan struct{}),
	}
}

func (w *genericWaiter[T]) ID() string {
	return w.id
}

func (w *genericWaiter[T]) Wait(ctx context.Context) (T, error) {
	var err error
	var data T
	select {
	case r, more := <-w.ch:
		if !more {
			// channel has been closed
			err = ErrWaiterClosed
			break
		}

		// msg recieved
		data = r.data
		err = r.err
	case <-w.doneCh:
		// close called on this waiter
		err = ErrWaiterClosed
		w.managerDoneCh <- w.id
	case <-ctx.Done():
		err = ctx.Err()
		w.managerDoneCh <- w.id
	}

	return data, err
}

func (w *genericWaiter[T]) Close() {
	close(w.doneCh)
}
