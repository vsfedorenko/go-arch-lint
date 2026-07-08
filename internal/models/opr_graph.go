package models

const (
	GraphTypeFlow GraphType = "flow"
	GraphTypeDI   GraphType = "di"
)

var GraphTypesValues = []string{
	GraphTypeFlow,
	GraphTypeDI,
}

const (
	GraphFormatSVG      GraphFormat = "svg"
	GraphFormatD2       GraphFormat = "d2"
	GraphFormatPlantUML GraphFormat = "plantuml"
	GraphFormatMermaid  GraphFormat = "mermaid"
)

var GraphFormatsValues = []string{
	GraphFormatSVG,
	GraphFormatD2,
	GraphFormatPlantUML,
	GraphFormatMermaid,
}

type (
	GraphType   = string
	GraphFormat = string

	CmdGraphIn struct {
		ProjectPath    string
		ArchFile       string
		Type           GraphType
		Format         GraphFormat
		OutFile        string
		Focus          string
		IncludeVendors bool
		ExportD2       bool
		OutputType     OutputType
	}

	CmdGraphOut struct {
		ProjectDirectory string      `json:"ProjectDirectory"`
		ModuleName       string      `json:"ModuleName"`
		OutFile          string      `json:"OutFile"`
		D2Definitions    string      `json:"D2Definitions"`
		GraphSource      string      `json:"GraphSource"`
		Format           GraphFormat `json:"Format"`
		ExportD2         bool        `json:"-"`
		IsTextOutput     bool        `json:"-"`
	}
)
