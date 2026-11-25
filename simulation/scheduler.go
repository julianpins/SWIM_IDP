package simulation

import (
	"time"
	"github.com/julianpins/SWIM_IDP/protocol"
)

type EventKind uint8

const (
	Deliver EventKind = iota 
	Expire      
	Inject
)

type Event struct {
	At    time.Duration
	Kind  EventKind
	Node  protocol.NodeID
	From  protocol.NodeID
	Msg   protocol.Message
	Timer protocol.Timer  
	Fault Fault            
}

type scheduler struct {
	now   time.Duration
	queue []Event
}

// push inserts e after all events with At <= e.At
func (s *scheduler) push(e Event)       {}
func (s *scheduler) pop() (Event, bool) { return Event{}, false }
