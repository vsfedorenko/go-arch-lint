// Package suppress extracts //go-arch-lint:ignore suppression
// directives from project source files and filters check results
// against them.
//
// The feature enables incremental adoption on legacy codebases: known
// violations are annotated in source with a suppression comment instead
// of being fixed immediately, and the check passes while new violations
// still fail it.
//
// Directive syntax (placed on the offending line or the line directly
// above it, in the same file):
//
//	//go-arch-lint:ignore         — suppress any violation on the line
//	//go-arch-lint:ignore beta    — suppress only violations whose
//	                                 dependency target matches "beta"
//
// A file-level variant suppresses every violation in the file:
//
//	//go-arch-lint:ignore-file
//
// Suppressed violations are counted and reported in the check output,
// so suppressions stay visible instead of silently hiding debt.
package suppress

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

// Directive kinds recognised in line comments. The comment text after
// "//" is matched against these (see extractDirective).
const (
	directiveIgnore     = "go-arch-lint:ignore"
	directiveIgnoreFile = "go-arch-lint:ignore-file"
)

// Index holds extracted suppression directives for the scanned project.
type Index struct {
	// fileLines maps absolute file path -> suppressed line number ->
	// dependency-target allow set (nil = any target).
	fileLines map[string]map[int]map[string]struct{}

	// files is the set of files fully suppressed by ignore-file.
	files map[string]struct{}
}

// NewEmptyIndex returns an index that suppresses nothing.
func NewEmptyIndex() *Index {
	return &Index{
		fileLines: map[string]map[int]map[string]struct{}{},
		files:     map[string]struct{}{},
	}
}

// NewIndexFromFiles reads suppression directives from every listed
// file. A file that disappeared between the project scan and this read
// is skipped: the linter must not crash on a tree changing mid-check.
func NewIndexFromFiles(paths []string) (*Index, error) {
	index := NewEmptyIndex()

	for _, path := range paths {
		if err := index.scanFile(path); err != nil {
			return nil, fmt.Errorf("failed to scan suppress directives in '%s': %w", path, err)
		}
	}

	return index, nil
}

// scanFile extracts directives from one file.
func (index *Index) scanFile(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	pending := pendingDirectives{}

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		text := scanner.Text()

		directive, argument, ok := extractDirective(text)
		if ok {
			if hasCodePrefix(text) {
				// Trailing directive (`import _ "x" //go-arch-lint:ignore`):
				// applies to its own line, not the next one.
				applyDirective(index, path, lineNumber, directive, argument)
				continue
			}

			pending.add(directive, argument)
			continue
		}

		if pending.hasAny() {
			// Standalone directives on preceding comment lines apply to
			// the next code line — e.g. the directive placed directly
			// above the offending import.
			pending.applyTo(index, path, lineNumber)
			pending = pendingDirectives{}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read '%s': %w", path, err)
	}

	return nil
}

// hasCodePrefix reports whether the line contains actual code before
// the comment holding the directive.
func hasCodePrefix(text string) bool {
	idx := strings.Index(text, "//")
	if idx < 0 {
		return false
	}

	return strings.TrimSpace(text[:idx]) != ""
}

// extractDirective recognises a suppression directive inside the line
// comment part of text and returns its kind plus the raw argument.
func extractDirective(text string) (directive string, argument string, ok bool) {
	// Directives live in line comments only (the go tooling
	// convention); block comments are not scanned.
	idx := strings.Index(text, "//")
	if idx < 0 {
		return "", "", false
	}

	comment := text[idx+2:]

	if kind, arg, ok := matchDirective(comment, directiveIgnoreFile); ok {
		return kind, arg, true
	}

	if kind, arg, ok := matchDirective(comment, directiveIgnore); ok {
		return kind, arg, true
	}

	return "", "", false
}

// matchDirective checks that comment starts with the given directive
// followed by end-of-comment or whitespace, so "ignore" never matches
// "ignore-file".
func matchDirective(comment, directive string) (string, string, bool) {
	if !strings.HasPrefix(comment, directive) {
		return "", "", false
	}

	if len(comment) > len(directive) {
		next := comment[len(directive)]
		if next != ' ' && next != '\t' {
			return "", "", false
		}
	}

	return directive, strings.TrimSpace(comment[len(directive):]), true
}

// pendingDirectives collects directives seen on consecutive comment
// lines so they can be applied to the next code line.
type pendingDirectives struct {
	entries []pendingEntry
}

type pendingEntry struct {
	directive string
	argument  string
}

func (p *pendingDirectives) hasAny() bool {
	return len(p.entries) > 0
}

func (p *pendingDirectives) add(directive, argument string) {
	p.entries = append(p.entries, pendingEntry{
		directive: directive,
		argument:  argument,
	})
}

func (p *pendingDirectives) applyTo(index *Index, path string, targetLine int) {
	for _, entry := range p.entries {
		applyDirective(index, path, targetLine, entry.directive, entry.argument)
	}
}

// applyDirective records a single directive for the target line.
func applyDirective(index *Index, path string, targetLine int, directive, argument string) {
	switch directive {
	case directiveIgnore:
		index.addLine(path, targetLine, argument)
	case directiveIgnoreFile:
		index.addFile(path)
	}
}

// addLine records a per-line suppression with an optional
// dependency-target filter.
func (index *Index) addLine(path string, line int, argument string) {
	if index.fileLines[path] == nil {
		index.fileLines[path] = map[int]map[string]struct{}{}
	}

	targets := index.fileLines[path][line]
	if strings.TrimSpace(argument) == "" {
		// No argument: the line's set resets to nil ("any target"),
		// which also subsumes previously recorded target filters.
		index.fileLines[path][line] = nil
		return
	}

	if targets == nil {
		targets = map[string]struct{}{}
	}

	for _, target := range strings.Fields(argument) {
		targets[target] = struct{}{}
	}

	index.fileLines[path][line] = targets
}

// addFile records a whole-file suppression.
func (index *Index) addFile(path string) {
	index.files[path] = struct{}{}
}

// IsFileSuppressed reports whether every violation in the file is
// suppressed by an ignore-file directive.
func (index *Index) IsFileSuppressed(path string) bool {
	_, ok := index.files[path]
	return ok
}

// IsLineSuppressed reports whether a violation at path:line with the
// given dependency target is suppressed. A directive without an
// argument suppresses any target; a directive with arguments
// suppresses only the listed targets. A target matches an argument
// either exactly or by its last path segment ("beta" matches
// "example.com/app/internal/beta").
func (index *Index) IsLineSuppressed(path string, line int, dependencyTarget string) bool {
	targets, ok := index.fileLines[path][line]
	if !ok {
		return false
	}

	if targets == nil {
		return true
	}

	if _, ok := targets[dependencyTarget]; ok {
		return true
	}

	if slash := strings.LastIndex(dependencyTarget, "/"); slash >= 0 {
		_, ok := targets[dependencyTarget[slash+1:]]
		return ok
	}

	return false
}

// HasDirectives reports whether the index holds anything at all —
// used to skip filtering work entirely on clean projects.
func (index *Index) HasDirectives() bool {
	return len(index.fileLines) > 0 || len(index.files) > 0
}
