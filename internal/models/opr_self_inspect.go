package models

import "github.com/vsfedorenko/go-arch-lint/internal/models/domain"

type (
	CmdSelfInspectIn struct {
		ProjectPath string
		ArchFile    string
	}

	CmdSelfInspectOut struct {
		ModuleName    string                        `json:"ModuleName"`
		RootDirectory string                        `json:"RootDirectory"`
		LinterVersion string                        `json:"LinterVersion"`
		Notices       []CmdSelfInspectOutAnnotation `json:"Notices"`
		Suggestions   []CmdSelfInspectOutAnnotation `json:"Suggestions"`
	}

	CmdSelfInspectOutAnnotation struct {
		Text      string           `json:"Text"`
		Reference domain.Reference `json:"Reference"`
	}
)
