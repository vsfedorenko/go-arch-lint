package checker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// Visibility enforces export-visibility rules: importing a component
// whose API is restricted by VisibleTo is a violation when the importer
// is not on the allow list. Works on the actual import graph — the only
// authoritative signal of "consumes the exported API" that is visible
// without type-checking (symbol-level tracking belongs to DeepScan).
type Visibility struct{}

func NewVisibility() *Visibility {
	return &Visibility{}
}

func (c *Visibility) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	if spec.Visibility == nil || len(spec.Visibility.Rules) == 0 {
		return models.CheckResult{}, nil
	}

	graph := buildComponentGraph(spec, projectFiles)

	// producer -> allowed consumers (the component itself is implicit).
	allowedByProducer := make(map[string]map[string]struct{}, len(spec.Visibility.Rules))
	for _, rule := range spec.Visibility.Rules {
		set, exists := allowedByProducer[rule.Component]
		if !exists {
			set = map[string]struct{}{}
			allowedByProducer[rule.Component] = set
		}
		set[rule.Component] = struct{}{}
		for _, a := range rule.Allowed {
			set[a] = struct{}{}
		}
	}

	// Graph adjacency is producer -> importers? No: buildComponentGraph
	// builds from = file owner, to = import owner, i.e. consumer ->
	// producer edges. Walk them directly: outer = consumer, key of
	// adjacency = consumer, witnesses point at producers.
	type violation struct {
		consumer  string
		producer  string
		witnesses []graphWitness
	}

	violations := map[string]*violation{}

	for consumer, producers := range graph {
		for producer, witness := range producers {
			allowed, restricted := allowedByProducer[producer]
			if !restricted {
				continue
			}
			if _, ok := allowed[consumer]; ok {
				continue
			}

			key := consumer + "->" + producer
			v, exists := violations[key]
			if !exists {
				v = &violation{consumer: consumer, producer: producer}
				violations[key] = v
			}
			v.witnesses = append(v.witnesses, witness)
		}
	}

	keys := make([]string, 0, len(violations))
	for k := range violations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := models.CheckResult{}

	for _, k := range keys {
		v := violations[k]
		sort.Slice(v.witnesses, func(i, j int) bool {
			return v.witnesses[i].file < v.witnesses[j].file
		})

		witness := v.witnesses[0]

		result.DependencyWarnings = append(result.DependencyWarnings, models.CheckArchWarningDependency{
			ComponentName: fmt.Sprintf(
				"%s may not consume '%s' (restricted API: visible to [%s])",
				v.consumer, v.producer,
				strings.Join(allowedSetToStrings(allowedByProducer[v.producer]), ", "),
			),
			FileRelativePath:   strings.TrimPrefix(witness.file, spec.RootDirectory.Value),
			FileAbsolutePath:   witness.file,
			ResolvedImportName: witness.imp.Name,
			Reference:          witness.imp.Reference,
		})
	}

	return result, nil
}

// allowedSetToStrings renders the allow set deterministically.
func allowedSetToStrings(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
