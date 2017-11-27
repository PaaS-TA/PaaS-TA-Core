package steps

// A step describes a synchronous action that can be interrupted.
//
// They can be composed arbitrarily assuming everyone's following the rules
// below.
type Step interface {
	// Perform synchronously performs something.
	//
	// If cancelled, it should return ErrCancelled (or an error wrapping it).
	Perform() error

	// Cancel asynchronously interrupts a running Perform().
	//
	// It can be called more than once, and should be idempotent.
	//
	// If the step is already completed, it is a no-op.
	//
	// If the step is cancelled, and then starts performing, it should
	// immediately cancel.
	Cancel()
}
