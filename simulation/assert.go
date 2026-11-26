package simulation

func assert(condition bool, message string) {
	if !condition {
		panic("assertion failed: " + message)
	}
}
