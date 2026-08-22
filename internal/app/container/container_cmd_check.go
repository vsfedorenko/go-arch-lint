package container

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/operations/check"
)

func (c *Container) commandCheck() (*cobra.Command, runner) {
	cmd := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "check project architecture against arch rules",
		Long:    "compare project *.go files with arch defined in spec file",
	}

	in := models.CmdCheckIn{
		ProjectPath: models.DefaultProjectPath,
		ArchFile:    models.DefaultArchFileName,
		MaxWarnings: 100,
	}

	cmd.PersistentFlags().StringVar(&in.ProjectPath, "project-path", in.ProjectPath, "absolute path to project directory")
	cmd.PersistentFlags().StringVar(&in.ArchFile, "arch-file", in.ArchFile, "arch file path")
	cmd.PersistentFlags().IntVar(&in.MaxWarnings, "max-warnings", in.MaxWarnings, "max number of warnings to output")
	cmd.PersistentFlags().StringVar(&in.BaselinePath, "baseline", "", "baseline file with known violations (only NEW violations fail the check)")
	cmd.PersistentFlags().BoolVar(&in.BaselineUpdate, "baseline-update", false, "record the current violations as the baseline (use with --baseline)")

	return cmd, func(act *cobra.Command) (any, error) {
		const warningsRangeMin = 1
		const warningsRangeMax = 32768

		if in.MaxWarnings < warningsRangeMin || in.MaxWarnings > warningsRangeMax {
			return nil, fmt.Errorf(
				"flag '%s' should be in range [%d .. %d]",
				"max-warnings",
				warningsRangeMin,
				warningsRangeMax,
			)
		}

		// Baseline pair lives on the check command (the root hook covers
		// the global output flags). Same shared rule set as the SDK path:
		// --baseline-update without --baseline must be an actionable
		// config error, not a plain check run that records nothing.
		flagPairs := models.CheckOptions{
			BaselinePath:   in.BaselinePath,
			BaselineUpdate: in.BaselineUpdate,
		}
		if err := flagPairs.ValidateFlagPairs(); err != nil {
			return nil, err
		}

		return c.commandCheckOperation().Behave(act.Context(), in)
	}
}

func (c *Container) commandCheckOperation() *check.Operation {
	return check.NewOperation(
		c.provideProjectInfoAssembler(),
		c.provideSpecAssembler(),
		c.provideSpecChecker(),
		c.provideReferenceRender(),
		c.flags.UseColors,
	)
}
