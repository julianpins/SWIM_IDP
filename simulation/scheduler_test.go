package simulation

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/julianpins/SWIM_IDP/protocol"
)

func testScheduler(t *testing.T, capacity int) *scheduler {
	t.Helper()
	s, err := newScheduler(capacity)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustPush(t *testing.T, s *scheduler, e Event) {
	t.Helper()
	if err := s.push(e); err != nil {
		t.Fatal(err)
	}
}

// tags an event via Node so pop order can be checked.
func tagged(at time.Duration, tag string) Event {
	return Event{At: at, Node: protocol.NodeID(tag)}
}

func popTags(t *testing.T, s *scheduler, n int) []string {
	t.Helper()
	var tags []string
	for i := 0; i < n; i++ {
		e, ok := s.pop()
		if !ok {
			t.Fatalf("pop %d: queue empty", i)
		}
		tags = append(tags, string(e.Node))
	}
	return tags
}

func TestNewSchedulerRejectsInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		if _, err := newScheduler(capacity); err == nil {
			t.Errorf("capacity %d: accepted", capacity)
		}
	}
}

func TestPopInTimeOrder(t *testing.T) {
	s := testScheduler(t, 8)
	mustPush(t, s, tagged(30, "c"))
	mustPush(t, s, tagged(10, "a"))
	mustPush(t, s, tagged(20, "b"))

	got := popTags(t, s, 3)
	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestEqualTimesPopFIFO(t *testing.T) {
	s := testScheduler(t, 8)
	mustPush(t, s, tagged(10, "first"))
	mustPush(t, s, tagged(5, "early"))
	mustPush(t, s, tagged(10, "second"))
	mustPush(t, s, tagged(10, "third"))

	got := popTags(t, s, 4)
	if want := []string{"early", "first", "second", "third"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestPushAtNowAfterPopStaysFIFO(t *testing.T) {
	s := testScheduler(t, 8)
	mustPush(t, s, tagged(10, "a"))
	mustPush(t, s, tagged(10, "b"))
	popTags(t, s, 1) // now = 10
	mustPush(t, s, tagged(10, "c"))

	got := popTags(t, s, 2)
	if want := []string{"b", "c"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestPopAdvancesNow(t *testing.T) {
	s := testScheduler(t, 8)
	mustPush(t, s, tagged(15, "a"))
	mustPush(t, s, tagged(40, "b"))

	s.pop()
	if s.now != 15 {
		t.Errorf("now %v after first pop, want 15", s.now)
	}
	s.pop()
	if s.now != 40 {
		t.Errorf("now %v after second pop, want 40", s.now)
	}
}

func TestAdvance(t *testing.T) {
	s := testScheduler(t, 8)
	s.advance(100)
	if s.now != 100 {
		t.Errorf("now %v, want 100", s.now)
	}
}

func TestEmptyQueue(t *testing.T) {
	s := testScheduler(t, 8)
	if _, ok := s.pop(); ok {
		t.Error("pop on empty queue returned ok")
	}
	if _, ok := s.peek(); ok {
		t.Error("peek on empty queue returned ok")
	}
}

func TestPeekDoesNotRemove(t *testing.T) {
	s := testScheduler(t, 8)
	mustPush(t, s, tagged(20, "b"))
	mustPush(t, s, tagged(10, "a"))

	e, ok := s.peek()
	if !ok || e.Node != "a" {
		t.Fatalf("peek = %v, %v; want a", e.Node, ok)
	}
	if s.now != 0 {
		t.Errorf("peek advanced now to %v", s.now)
	}
	if got := popTags(t, s, 2); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("order after peek %v", got)
	}
}

func TestPushFullQueue(t *testing.T) {
	s := testScheduler(t, 2)
	mustPush(t, s, tagged(1, "a"))
	mustPush(t, s, tagged(2, "b"))
	if err := s.push(tagged(3, "c")); !errors.Is(err, errQueueFull) {
		t.Errorf("push on full queue: %v, want errQueueFull", err)
	}
}

func TestQueueDoesNotGrow(t *testing.T) {
	s := testScheduler(t, 4)
	for i := 0; i < 4; i++ {
		mustPush(t, s, tagged(time.Duration(4-i), "x"))
	}
	if cap(s.queue) != 4 {
		t.Errorf("queue capacity %d, want 4", cap(s.queue))
	}
}

func TestPushInPastPanics(t *testing.T) {
	s := testScheduler(t, 8)
	s.advance(50)
	defer func() {
		if recover() == nil {
			t.Error("push in the past did not panic")
		}
	}()
	s.push(tagged(10, "late"))
}

func TestAdvanceBackwardsPanics(t *testing.T) {
	s := testScheduler(t, 8)
	s.advance(50)
	defer func() {
		if recover() == nil {
			t.Error("advance backwards did not panic")
		}
	}()
	s.advance(10)
}
