package protocol

// panics on a broken invariant (programmer error); expected failures return errors.
func assert(condition bool, message string) {
	if !condition {
		panic("assertion failed: " + message)
	}
}
