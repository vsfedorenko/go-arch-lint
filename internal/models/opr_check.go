package models

import "github.com/vsfedorenko/go-arch-lint/internal/models/domain"

type (
	CmdCheckIn struct {
		ProjectPath string
		ArchFile    string
		MaxWarnings int
	}

	CmdCheckOut struct {
		DocumentNotices        []CheckNotice                `json:"ExecutionWarnings"`
		ArchHasWarnings        bool                         `json:"ArchHasWarnings"`
		ArchWarningsDependency []CheckArchWarningDependency `json:"ArchWarningsDeps"`
		ArchWarningsMatch      []CheckArchWarningMatch      `json:"ArchWarningsNotMatched"`
		ArchWarningsDeepScan   []CheckArchWarningDeepscan   `json:"ArchWarningsDeepScan"`
		ArchWarningsNaming     []CheckArchWarningNaming     `json:"ArchWarningsNaming"`
		OmittedCount           int                          `json:"OmittedCount"`
		ModuleName             string                       `json:"ModuleName"`
		Qualities              []CheckQuality               `json:"Qualities"`
	}

	CheckQuality struct {
		ID   string `json:"ID"`
		Used bool   `json:"Used"`
		Name string `json:"-"`
		Hint string `json:"-"`
	}

	CheckNotice struct {
		Text              string `json:"Text"`
		File              string `json:"File"`
		Line              int    `json:"Line"`
		Column            int    `json:"Offset"`
		SourceCodePreview []byte `json:"-"`
	}

	CheckArchWarningDependency struct {
		ComponentName      string           `json:"ComponentName"`
		FileRelativePath   string           `json:"FileRelativePath"`
		FileAbsolutePath   string           `json:"FileAbsolutePath"`
		ResolvedImportName string           `json:"ResolvedImportName"`
		Reference          domain.Reference `json:"Reference"`
	}

	CheckArchWarningNaming struct {
		PackageName      string `json:"PackageName"`
		PackagePath      string `json:"PackagePath"`
		FileRelativePath string `json:"FileRelativePath"`
		FileAbsolutePath string `json:"-"`
		FilesCount       int    `json:"FilesCount"`
	}

	CheckArchWarningMatch struct {
		FileRelativePath string           `json:"FileRelativePath"`
		FileAbsolutePath string           `json:"FileAbsolutePath"`
		Reference        domain.Reference `json:"-"`
	}

	CheckArchWarningDeepscan struct {
		Gate       DeepscanWarningGate       `json:"Gate"`
		Dependency DeepscanWarningDependency `json:"Dependency"`
		Target     DeepscanWarningTarget     `json:"Target"`
	}

	DeepscanWarningGate struct {
		ComponentName string           `json:"ComponentName"` // operations
		MethodName    string           `json:"MethodName"`    // NewOperation
		Definition    domain.Reference `json:"Definition"`    // internal/glue/code/line_count.go:54
		RelativePath  string           `json:"-"`             // internal/glue/code/line_count.go:54
	}

	DeepscanWarningDependency struct {
		ComponentName     string           `json:"ComponentName"` // repository
		Name              string           `json:"Name"`          // micro.ViewRepository
		InjectionAST      string           `json:"InjectionAST"`  // c.provideMicroViewRepository()
		Injection         domain.Reference `json:"Injection"`     // internal/app/container/container_cmd_mapping.go:15
		InjectionPath     string           `json:"-"`             // internal/app/container/container_cmd_mapping.go:15
		SourceCodePreview []byte           `json:"-"`
	}

	DeepscanWarningTarget struct {
		Definition   domain.Reference `json:"Definition"`
		RelativePath string           `json:"-"` // internal/app/container/container_cmd_mapping.go:15
	}

	CheckResult struct {
		DependencyWarnings []CheckArchWarningDependency
		MatchWarnings      []CheckArchWarningMatch
		DeepscanWarnings   []CheckArchWarningDeepscan
		NamingWarnings     []CheckArchWarningNaming
	}
)

func (cr *CheckResult) Append(another CheckResult) {
	cr.DependencyWarnings = append(cr.DependencyWarnings, another.DependencyWarnings...)
	cr.MatchWarnings = append(cr.MatchWarnings, another.MatchWarnings...)
	cr.DeepscanWarnings = append(cr.DeepscanWarnings, another.DeepscanWarnings...)
	cr.NamingWarnings = append(cr.NamingWarnings, another.NamingWarnings...)
}

func (cr *CheckResult) HasNotices() bool {
	if len(cr.DependencyWarnings) > 0 {
		return true
	}
	if len(cr.MatchWarnings) > 0 {
		return true
	}
	if len(cr.DeepscanWarnings) > 0 {
		return true
	}
	if len(cr.NamingWarnings) > 0 {
		return true
	}

	return false
}
