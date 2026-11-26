package simulation

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

type EventKind uint8

const (
	Deliver EventKind = iota
	Expire
	Inject
)

// no pointers, funcs or channels: Trace.Hash would differ between runs.
type Event struct {
	At    time.Duration
	Kind  EventKind
	Node  protocol.NodeID
	From  protocol.NodeID
	Msg   protocol.Message
	Timer protocol.Timer
	Fault Fault
}

// queue sorted by At descending: next event is last, equal At popped FIFO.
type scheduler struct {
	now      time.Duration
	queue    []Event
	capacity int
}

func newScheduler(capacity int) (*scheduler, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("scheduler: capacity must be positive, got %d", capacity)
	}
	return &scheduler{queue: make([]Event, 0, capacity), capacity: capacity}, nil
}

var errQueueFull = errors.New("scheduler: event queue full")

// inserts e behind all events with the same or an earlier At.
func (s *scheduler) push(e Event) error {
	assert(e.At >= s.now, "push: event in the past")
	if len(s.queue) == s.capacity {
		return errQueueFull
	}
	i := sort.Search(len(s.queue), func(i int) bool { return s.queue[i].At <= e.At })
	s.queue = append(s.queue, Event{})
	copy(s.queue[i+1:], s.queue[i:])
	s.queue[i] = e
	return nil
}

// removes the next event and advances now to its time.
func (s *scheduler) pop() (Event, bool) {
	if len(s.queue) == 0 {
		return Event{}, false
	}
	last := len(s.queue) - 1
	e := s.queue[last]
	s.queue = s.queue[:last]
	assert(e.At >= s.now, "pop: time went backwards")
	s.now = e.At
	return e, true
}

// returns the next event without removing it.
func (s *scheduler) peek() (Event, bool) {
	if len(s.queue) == 0 {
		return Event{}, false
	}
	return s.queue[len(s.queue)-1], true
}

// moves now forward to t without an event.
func (s *scheduler) advance(t time.Duration) {
	assert(t >= s.now, "advance: time went backwards")
	s.now = t
}
