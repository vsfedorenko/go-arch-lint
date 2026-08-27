package models

type (
	// CmdExplainIn is the input of the `explain` command: one import
	// path to dissect against the assembled spec.
	CmdExplainIn struct {
		ProjectPath string
		ArchFile    string
		ImportPath  string
	}

	// CmdExplainOut explains how the spec treats one import path: its
	// classification, the owning component (project imports), the
	// verdict for EVERY component, and actual usage sites found in the
	// project scan.
	CmdExplainOut struct {
		ModuleName string `json:"ModuleName"`

		// ImportPath is the queried path, echoed back.
		ImportPath string `json:"ImportPath"`

		// ImportType is the classification: "std" | "project" | "vendor".
		ImportType string `json:"ImportType"`

		// OwnerComponent is the component whose paths contain this
		// import (project imports only, empty otherwise).
		OwnerComponent string `json:"OwnerComponent"`

		// Verdicts holds one entry per declared component.
		Verdicts []CmdExplainVerdict `json:"Verdicts"`

		// Usages lists actual import sites found in the project scan
		// (file, line, importing component), capped for display.
		Usages []CmdExplainUsage `json:"Usages"`

		// OmittedUsages is the number of usage sites beyond the cap.
		OmittedUsages int `json:"OmittedUsages"`
	}

	// CmdExplainVerdict is the allow/deny decision for one component,
	// with the exact rule that produced it and a concrete fix when
	// denied.
	CmdExplainVerdict struct {
		// Component name.
		Component string `json:"Component"`

		// Allowed is the decision under the current spec.
		Allowed bool `json:"Allowed"`

		// Rule names the rule that produced the decision, e.g.
		// "stdlib imports are always allowed" or
		// "Use(core) in arch.go:12".
		Rule string `json:"Rule"`

		// Fix is a concrete spec edit that would allow the import for
		// this component (denied verdicts only).
		Fix string `json:"Fix,omitempty"`
	}

	// CmdExplainUsage is one actual import site of the queried path.
	CmdExplainUsage struct {
		File      string `json:"File"`
		Line      int    `json:"Line"`
		Component string `json:"Component"`
	}
)
