package models

type CheckOptions struct {
	ProjectPath string
	MaxWarnings int
	UseColors   bool
	Format      Format

	// OutputType selects the wrapper rendering of check results:
	// "ascii" (human-readable, default) or "json" (the {Type, Payload}
	// wrapper model). Independent of Format, which drives the dedicated
	// machine formats (json/sarif/junit/...) of the violation payload.
	OutputType OutputType

	// OutputJSONOneLine renders the json output type as a single-line
	// payload (no indentation). Only meaningful with OutputType json;
	// the public API rejects it otherwise (a silently ignored flag is
	// worse than an error, see upstream issue #62 class).
	OutputJSONOneLine bool

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
