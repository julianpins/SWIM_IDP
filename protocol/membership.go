package protocol

type State uint8

const (
	Alive State = iota
	Suspect
	Dead
)

type Member struct {
	ID          NodeID
	State       State
	Incarnation uint64
}

// a change in this node's view of Member; no timestamp, the environment adds it.
type Change struct {
	Member      NodeID
	Known       bool // false: Member was not in the table before; From is meaningless
	From        State
	To          State
	Incarnation uint64
}

// paper Section 4.2: whether update replaces the current entry.
func overrides(u Update, m Member) bool {
	switch m.State {
	case Alive:
		switch u.State {
		case Alive:
			return u.Incarnation > m.Incarnation
		case Suspect:
			return u.Incarnation >= m.Incarnation
		case Dead:
			return true
		}
	case Suspect:
		switch u.State {
		case Alive, Suspect:
			return u.Incarnation > m.Incarnation
		case Dead:
			return true
		}
	case Dead:
		return false
	}
	assert(false, "overrides: unknown state")
	return false
}

// returns the previous state and whether anything changed. Unknown members take any update.
func (n *Node) applyUpdate(u Update) (previous State, changed bool) {
	assert(u.Member != "", "applyUpdate: empty member")
	assert(u.Member != n.id, "applyUpdate: updates about self go to refute")
	assert(u.State <= Dead, "applyUpdate: unknown state")

	m, known := n.members[u.Member]
	if !known {
		n.members[u.Member] = &Member{ID: u.Member, State: u.State, Incarnation: u.Incarnation}
		return u.State, true
	}
	if !overrides(u, *m) {
		return m.State, false
	}
	previous = m.State
	m.State, m.Incarnation = u.State, u.Incarnation
	return previous, true
}
