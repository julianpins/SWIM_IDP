package simulation

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/julianpins/SWIM_IDP/protocol"
)

// one processed event and the view changes it caused, in order; Event carries At and Node.
type Entry struct {
	Event   Event
	Changes []protocol.Change
}

type Trace []Entry

// sha256 over the Go-syntax form of every entry.
func (t Trace) Hash() [32]byte {
	h := sha256.New()
	for _, e := range t {
		fmt.Fprintf(h, "%#v\n", e)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// one line per event, each change indented below it; changesOnly prints only the changes.
func (t Trace) Format(w io.Writer, changesOnly bool) error {
	for _, entry := range t {
		e := entry.Event
		node := string(e.Node)
		if node == "" {
			node = "-"
		}
		if !changesOnly {
			line := fmt.Sprintf("%12.6fs  %-4s  %-8s %s", e.At.Seconds(), node, e.Kind, eventDetail(e))
			if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
				return err
			}
		}
		for _, c := range entry.Changes {
			if _, err := fmt.Fprintf(w, "%12.6fs  %-4s    %s\n", e.At.Seconds(), node, changeDetail(c)); err != nil {
				return err
			}
		}
	}
	return nil
}

func eventDetail(e Event) string {
	switch e.Kind {
	case Deliver:
		return messageDetail(e.Msg) + " from " + string(e.From)
	case Expire:
		return timerDetail(e.Timer)
	case Inject:
		return faultDetail(e.Fault)
	}
	return ""
}

// e.g. "PingReq#3 c [b Suspect/0]".
func messageDetail(m protocol.Message) string {
	s := fmt.Sprintf("%s#%d", m.Kind, m.Probe)
	if m.Target != "" {
		s += " " + string(m.Target)
	}
	if len(m.Updates) > 0 {
		updates := make([]string, len(m.Updates))
		for i, u := range m.Updates {
			updates[i] = fmt.Sprintf("%s %s/%d", u.Member, u.State, u.Incarnation)
		}
		s += " [" + strings.Join(updates, ", ") + "]"
	}
	return s
}

// e.g. "ProbeTimeout#4 c" or "SuspicionTimeout c/0".
func timerDetail(t protocol.Timer) string {
	switch t.Kind {
	case protocol.PeriodEnd:
		return t.Kind.String()
	case protocol.SuspicionTimeout:
		return fmt.Sprintf("%s %s/%d", t.Kind, t.Member, t.Incarnation)
	case protocol.ProbeTimeout:
		return fmt.Sprintf("%s#%d %s", t.Kind, t.Probe, t.Member)
	}
	return fmt.Sprintf("%s#%d", t.Kind, t.Probe)
}

func faultDetail(f Fault) string {
	switch f.Kind {
	case Crash, Restart:
		return fmt.Sprintf("%s %s", f.Kind, f.Node)
	case Partition:
		return fmt.Sprintf("%s %v", f.Kind, f.Groups)
	}
	return f.Kind.String()
}

// e.g. "b: Alive -> Suspect (inc 0)"; "new" for a member not known before.
func changeDetail(c protocol.Change) string {
	from := c.From.String()
	if !c.Known {
		from = "new"
	}
	return fmt.Sprintf("%s: %s -> %s (inc %d)", c.Member, from, c.To, c.Incarnation)
}
