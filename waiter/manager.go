package waiter

import (
	"context"
	"errors"
	"sync"
)

const (
	// The default number of attempts to re-generate an id on collision
	defaultMaxCollisions = 5
)

var (
	ErrTooManyCollisions = errors.New("too many id collisions")
	ErrManagerShutdown   = errors.New("manager is shutdown")

	errKeyExists = errors.New("id exists")
	errNoExists  = errors.New("id does not exist")
)

type response[T any] struct {
	id   string
	data T
	err  error
}

type options struct {
	idGenerator   IDGenerator
	maxCollisions int
}

type Option func(option *options)

func noopOptionFunc(option *options) {}

func WithIDGenerator(gen IDGenerator) Option {
	if gen == nil {
		return noopOptionFunc
	}

	return func(options *options) {
		options.idGenerator = gen
	}
}

func WithMaxCollisions(num int) Option {
	if num < 1 {
		return noopOptionFunc
	}

	return func(options *options) {
		options.maxCollisions = num
	}
}

// Manager builds waiters and delivers messages to waiters from async publishers
type Manager[T any] struct {
	// idGenerator is responsible for generating unique waiter IDs
	idGenerator IDGenerator

	// waiters holds a waiter's id and the channel to send responses on
	waiters map[string]chan *response[T]

	// mu ensures only one goroutine is manipulating the waiters map at a time
	mu sync.Mutex

	// responseCh recieves all responses meant for waiting waiters
	responseCh chan *response[T]

	// doneWaiterCh ids sent on this channel allow the manager to cleanup done waiters
	doneWaiterCh chan string

	// managerShutdownCh will be closed when manager is shutting down and performing cleanup
	managerShutdownCh chan struct{}

	// maxCollisions the max number of id generation collisions allowed prior to error
	// when creating a new waiter
	maxCollisions int
}

func NewManager[T any](opts ...Option) *Manager[T] {
	var options options

	options.maxCollisions = defaultMaxCollisions

	for _, opt := range opts {
		opt(&options)
	}

	if options.idGenerator == nil {
		options.idGenerator = &UUIDGenerator{}
	}

	return &Manager[T]{
		idGenerator:       options.idGenerator,
		waiters:           map[string]chan *response[T]{},
		responseCh:        make(chan *response[T]),
		doneWaiterCh:      make(chan string),
		managerShutdownCh: make(chan struct{}),
		maxCollisions:     options.maxCollisions,
	}
}

func (w *Manager[T]) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				// ensure are no more sends
				close(w.managerShutdownCh)

				// inform the waiters
				w.closeWaiters()
				return
			case r := <-w.responseCh:
				waiterCh, err := w.removeWaiter(r.id)
				if errors.Is(err, errNoExists) {
					continue
				}

				// to prevent blocking, waiterCh should be buffered with
				// size of 1 (each waiterCh should only ever receive 1 msg)
				// (alternatively use a goroutine)
				waiterCh <- r
			case id := <-w.doneWaiterCh:
				// the waiter has been closed or canceled
				w.removeWaiter(id)
			}
		}
	}()
}

func (w *Manager[T]) Send(id string, data T, err error) error {
	// if the manager is shutdown, return immediatly
	select {
	case <-w.managerShutdownCh:
		return ErrManagerShutdown
	default:
	}

	// check again if the manager is shutdown, return if so
	// otherwise write the message to the proper channel
	select {
	case <-w.managerShutdownCh:
		return ErrManagerShutdown
	case w.responseCh <- &response[T]{
		id:   id,
		data: data,
		err:  err,
	}:
	}

	return nil
}

// NewWaiter creates a waiter with a unique ID. A call to Wait() will complete when
// a response is published using this ID
func (w *Manager[T]) NewWaiter() (Waiter[T], error) {
	// if the manager is shutdown, error out
	select {
	case <-w.managerShutdownCh:
		return nil, ErrManagerShutdown
	default:
	}

	// otherwise setup the new waiter
	var waiterCh chan *response[T]
	for i := 0; i < w.maxCollisions; i++ {
		id, err := w.idGenerator.ID()
		if err != nil {
			return nil, err
		}

		waiterCh, err = w.addWaiter(id)
		if err == nil {
			return newGenericWaiter(id, waiterCh, w.doneWaiterCh), nil
		}
	}

	return nil, ErrTooManyCollisions
}

func (w *Manager[T]) addWaiter(id string) (chan *response[T], error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.waiters[id]; ok {
		return nil, errKeyExists
	}

	// create a buffered channel of size 1 so that the loop initiated by w.Start()
	// will not have to wait for this channel to be read (making it non-blocking)
	ch := make(chan *response[T], 1)

	w.waiters[id] = ch
	return ch, nil
}

func (w *Manager[T]) removeWaiter(id string) (chan *response[T], error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	waiterCh, ok := w.waiters[id]
	if !ok {
		return nil, errNoExists
	}

	delete(w.waiters, id)

	return waiterCh, nil
}

func (w *Manager[T]) closeWaiters() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, ch := range w.waiters {
		close(ch)
	}
}
