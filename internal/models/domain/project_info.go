package domain

type (
	Project struct {
		Directory      string
		GoArchFilePath string
		GoModFilePath  string
		ModuleName     string

		// WorkspaceModules lists the sibling modules declared in a root
		// go.work (`use` directives): absolute directory plus module path.
		// The root module itself is NOT listed — it is already covered by
		// ModuleName/Directory. Empty when the project has no go.work.
		WorkspaceModules []WorkspaceModule
	}

	// WorkspaceModule is one `use` entry of a go.work file.
	WorkspaceModule struct {
		Dir  string // absolute directory of the module
		Path string // module path exactly as declared in its go.mod
	}
)
