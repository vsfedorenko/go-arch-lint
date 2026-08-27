package container

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/operations/explain"
)

func (c *Container) commandExplain() (*cobra.Command, runner) {
	cmd := &cobra.Command{
		Use:   "explain <import>",
		Short: "explain how the spec treats one import path",
		Long: "show the import's classification (std/project/vendor), the owning component, " +
			"the allow/deny verdict for every component with the exact rule that produced it, " +
			"and the actual import sites found in the project",
		Args: cobra.ExactArgs(1),
	}

	in := models.CmdExplainIn{
		ProjectPath: models.DefaultProjectPath,
		ArchFile:    models.DefaultArchFileName,
	}

	cmd.PersistentFlags().StringVar(&in.ProjectPath, "project-path", in.ProjectPath, "absolute path to project directory")
	cmd.PersistentFlags().StringVar(&in.ArchFile, "arch-file", in.ArchFile, "arch file path")

	return cmd, func(act *cobra.Command) (any, error) {
		in.ImportPath = act.Flags().Args()[0]
		if in.ImportPath == "" {
			return nil, fmt.Errorf("explain requires an import path argument")
		}

		return c.commandExplainOperation().Behave(act.Context(), in)
	}
}

func (c *Container) commandExplainOperation() *explain.Operation {
	return explain.NewOperation(
		c.provideSpecAssembler(),
		c.provideProjectInfoAssembler(),
		c.provideProjectFilesResolver(),
		c.provideProjectFilesScanner().StdPackages(),
	)
}
