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

// ValidateFlagPairs rejects incoherent flag combinations with an
// actionable error instead of letting them silently no-op. One rule set
// shared by BOTH entry paths — the public archlint.Run SDK path and the
// cobra tree the scaffolded runner drives via MustRunCLI — so a scaffolded
// `check --baseline-update` (no --baseline) fails the same way the SDK
// does, instead of running a plain check that records nothing.
func (o CheckOptions) ValidateFlagPairs() error {
	if o.BaselineUpdate && o.BaselinePath == "" {
		return NewConfigError("--baseline-update requires --baseline <file> to know where to record the fingerprints")
	}

	if o.OutputJSONOneLine {
		jsonOutput := o.OutputType == OutputTypeJSON

		// --format json (flat violation array) is also JSON on stdout;
		// compacting it is meaningful there, so accept the combination.
		if o.Format == FormatJSON {
			jsonOutput = true
		}

		if !jsonOutput {
			return NewConfigError("--output-json-one-line only affects json output: add --output-type=json (or --json), or drop the flag")
		}
	}

	return nil
}
