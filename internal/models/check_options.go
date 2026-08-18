package models

type CheckOptions struct {
	ProjectPath string
	MaxWarnings int
	UseColors   bool
	Format      Format

	// BaselinePath enables the incremental adoption mode: violations
	// whose fingerprints are recorded in the baseline file are tolerated
	// (known debt), only NEW violations fail the check. Empty disables
	// the mode.
	BaselinePath string

	// BaselineUpdate switches the baseline mode from "compare" to
	// "record": the check writes the current full fingerprint set to
	// BaselinePath instead of comparing against it. Requires
	// BaselinePath.
	BaselineUpdate bool
}
