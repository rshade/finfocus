package greenops

// constError is an immutable error type for sentinel errors.
// It implements the error interface and provides compile-time safety.
type constError string

func (e constError) Error() string { return string(e) }

// Error types for equivalency calculations.
// These are sentinel errors that can be compared with errors.Is().
var (
	// ErrInvalidUnit indicates an unrecognized carbon unit.
	// This error is returned when NormalizeToKg receives an unknown unit string.
	ErrInvalidUnit = constError("invalid carbon unit")

	// ErrNegativeValue indicates a negative carbon value.
	// Carbon emissions cannot be negative.
	ErrNegativeValue = constError("negative carbon value")

	// ErrCalculationOverflow indicates a value too large to calculate safely.
	// This is a safety check to prevent floating point overflow.
	ErrCalculationOverflow = constError("calculation overflow")
)
