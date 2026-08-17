package models

const (
	MappingSchemeGrouped MappingScheme = "grouped"
	MappingSchemeList    MappingScheme = "list"
)

var MappingSchemesValues = []string{
	MappingSchemeList,
	MappingSchemeGrouped,
}

type (
	MappingScheme = string

	CmdMappingIn struct {
		ProjectPath string
		ArchFile    string
		Scheme      MappingScheme
	}

	CmdMappingOut struct {
		ProjectDirectory string                 `json:"ProjectDirectory"`
		ModuleName       string                 `json:"ModuleName"`
		MappingGrouped   []CmdMappingOutGrouped `json:"MappingGrouped"`
		MappingList      []CmdMappingOutList    `json:"MappingList"`
		Scheme           MappingScheme          `json:"-"`
	}

	CmdMappingOutGrouped struct {
		ComponentName string
		FileNames     []string

		// Coupling measured on the actual import graph (0 when metrics are
		// not computed — backward-compatible JSON).
		Coupling *ComponentCoupling `json:",omitempty"`
	}

	// ComponentCoupling surfaces dependency metrics per component:
	// fan-out (efferent), fan-in (afferent) and the Robert C. Martin
	// stability ratio I = Ce / (Ca + Ce).
	ComponentCoupling struct {
		Name         string  `json:"Name"`
		OutboundDeps int     `json:"OutboundDeps"`
		InboundDeps  int     `json:"InboundDeps"`
		Stability    float64 `json:"Stability"`
	}

	CmdMappingOutList struct {
		FileName      string
		ComponentName string
	}
)
