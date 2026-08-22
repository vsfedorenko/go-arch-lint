package deepscan_test

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/checker/deepscan"
)

// fixtureProject is the nested test module with a rich set of injectable
// constructors: named/aliased interfaces, spread params, channels, maps,
// shared-package interfaces and anonymous interfaces.
func fixtureProject(t *testing.T) string {
	t.Helper()

	_, callerDir, _, _ := runtime.Caller(0) //nolint:dogsled // runtime.Caller returns 4 values; only the dir matters
	return filepath.Join(filepath.Dir(callerDir), "test", "project")
}

func fixtureCriteria(t *testing.T) deepscan.Criteria {
	t.Helper()

	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(filepath.Join(fixtureProject(t), "internal", "operations")),
		deepscan.WithAnalyseScope(filepath.Join(fixtureProject(t), "internal")),
	)
	require.NoError(t, err)

	return criteria
}

// methodNames returns the flat list of discovered method names in scan order.
func methodNames(methods []deepscan.InjectionMethod) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, method.Name)
	}

	return names
}

// findMethod returns the discovered method by name.
func findMethod(t *testing.T, methods []deepscan.InjectionMethod, name string) deepscan.InjectionMethod {
	t.Helper()

	for _, method := range methods {
		if method.Name == name {
			return method
		}
	}

	require.Failf(t, "method %q not found in scan result: %v", name, methodNames(methods))
	return deepscan.InjectionMethod{}
}

func implementationCodes(gate deepscan.Gate) []string {
	codes := make([]string, 0, len(gate.Implementations))
	for _, impl := range gate.Implementations {
		codes = append(codes, impl.Injector.CodeName)
	}

	return codes
}

func TestUsagesDiscoversInjectableMethods(t *testing.T) {
	// act
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))

	// assert: every public method with an interface-typed param is found,
	// across all supported shapes (basic, named, alias, dual, spread,
	// variadic, slice/array element, shared package, anonymous).
	require.NoError(t, err)

	expectedMethods := []string{
		// basic constructors
		"NewProcessorBasic1",
		"NewProcessorNames1",
		"NewProcessorAlias1",
		"NewProcessorBasicDual1",
		"NewProcessorBasicSpreadNames1",
		"NewProcessorBasicSpreadNamesAnonim1",
		"NewProcessorBasicSpreadTypes1",
		"NewProcessorBasicSlice1",
		"NewProcessorBasicArray1",
		// public/private combinations
		"PublicFindInPrivate2",
		"PublicFindInPublic2",
		// receiver variants
		"DoWork3",
		"DoCopyWork3",
		// read-direction channels are injectable
		"VisibleReadFrom4",
		"VisibleReadFromSpread4",
		"VisibleReadFromSlice4",
		"VisibleReadFromArray4",
		"VisiblePotentiallyReadFrom4",
		"VisiblePotentiallyReadFromSpread4",
		"VisiblePotentiallyReadFromArray4",
		// method on a type (stdlib interface param)
		"Bytes",
		// map values
		"MapSimple5",
		"MapSlice5",
		"MapChanVisible5",
		"MapChanPotentionallyVisible5",
		"MapInside5",
		// interface from a shared package
		"SharedVisible6",
		// anonymous interfaces
		"AnonAnyInterface7",
		"AnonMethodInterface7",
	}
	assert.ElementsMatch(t, expectedMethods, methodNames(methods))
}

func TestUsagesSkipsNonInjectableMethods(t *testing.T) {
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	// private methods can never be called from another component
	assert.NotContains(t, methodNames(methods), "privateFindInPrivate2")

	// write-only channels receive from our side, nothing is injected
	assert.NotContains(t, methodNames(methods), "InvisibleWriteTo4")

	// no interface param at all
	assert.NotContains(t, methodNames(methods), "NewIntSpread")

	// return-type-only usage is not a dependency gate
	assert.NotContains(t, methodNames(methods), "InvisibleFetchFrom4")
}

func TestUsagesResolvesInterfaceAliasParam(t *testing.T) {
	// `func NewProcessorAlias1(myFetcher myFetcherAlias)` — the param type is
	// a type alias to an interface and must resolve to the aliased interface
	// definition, not be silently skipped.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorAlias1")
	require.Len(t, method.Gates, 1)

	gate := method.Gates[0]
	assert.Equal(t, "myFetcher", gate.Interface.Name)
	assert.True(t, strings.HasSuffix(gate.Interface.Definition.Place.File, "1_basic_constructor.go"))
	assert.Equal(t, 4, gate.Interface.Definition.Place.Line)

	// the fixture injects a *repository.Memory through a provider function
	require.Len(t, gate.Implementations, 1)
	impl := gate.Implementations[0]
	assert.Equal(t, "t4FunctionPassProvideMemoryRepository()", impl.Injector.CodeName)
	assert.Equal(t, "Memory", impl.Target.StructName)
	assert.True(t, strings.HasSuffix(impl.Injector.MethodDefinition.Place.File, "di/1_basic.go"))
	assert.True(t, strings.HasSuffix(impl.Target.Definition.Place.File, "mem.go"))
}

func TestUsagesFindsDirectImplementation(t *testing.T) {
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasic1")
	require.Len(t, method.Gates, 1)

	gate := method.Gates[0]
	assert.Equal(t, "myFetcher", gate.ParamName)
	assert.Equal(t, 0, gate.Index)
	assert.False(t, gate.IsVariadic)

	require.Len(t, gate.Implementations, 1)
	assert.Equal(t, []string{"repository.NewMemory()"}, implementationCodes(gate))
	assert.Equal(t, "Memory", gate.Implementations[0].Target.StructName)
}

func TestUsagesTracksEveryNamedParamOfDualGate(t *testing.T) {
	// fetcher1 (alias type) and fetcher2 (defined type) are separate gates,
	// each with its own injected argument.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasicDual1")
	require.Len(t, method.Gates, 2)

	assert.Equal(t, "fetcher1", method.Gates[0].ParamName)
	assert.Equal(t, 0, method.Gates[0].Index)
	assert.Equal(t, []string{"memRepo"}, implementationCodes(method.Gates[0]))

	assert.Equal(t, "fetcher2", method.Gates[1].ParamName)
	assert.Equal(t, 1, method.Gates[1].Index)
	assert.Equal(t, []string{"anotherMemRepo"}, implementationCodes(method.Gates[1]))
}

func TestUsagesSkipsBlankParams(t *testing.T) {
	// `func New(fetcher1, _, _, fetcher4 myFetcherNamed)` — placeholder
	// params cannot receive an injection, so only fetcher1/fetcher4 are gates.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasicSpreadNamesAnonim1")
	require.Len(t, method.Gates, 2)
	assert.Equal(t, "fetcher1", method.Gates[0].ParamName)
	assert.Equal(t, 0, method.Gates[0].Index)
	assert.Equal(t, "fetcher4", method.Gates[1].ParamName)
	assert.Equal(t, 3, method.Gates[1].Index)
}

func TestUsagesCollectsAllVariadicArguments(t *testing.T) {
	// NewProcessorBasicSpreadTypes1(fetchers ...myFetcherNamed) is called
	// with three distinct arguments — each is an injection point.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasicSpreadTypes1")
	require.Len(t, method.Gates, 1)

	gate := method.Gates[0]
	assert.True(t, gate.IsVariadic)

	codes := implementationCodes(gate)
	require.Len(t, codes, 3)
	assert.Contains(t, codes, "repository.NewMemory()")
	assert.Contains(t, codes, "sameRepo")
	assert.Contains(t, codes, "func() *repository.Memory {\n\treturn repository.NewMemory()\n}()")
}

func TestUsagesResolvesSharedPackageInterface(t *testing.T) {
	// SharedVisible6(s shared.Repository) — the interface lives in another
	// package of the same module; its definition must point there.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	method := findMethod(t, methods, "SharedVisible6")
	require.Len(t, method.Gates, 1)

	gate := method.Gates[0]
	assert.Equal(t, "Repository", gate.Interface.Name)
	assert.True(t, strings.HasSuffix(gate.Interface.Definition.Place.File, "shared/interfaces.go"))
	assert.Equal(t, 4, gate.Interface.Definition.Place.Line)
}

func TestUsagesReportsSliceOfAliasElementGate(t *testing.T) {
	// fetchers []PublicFetcherForDI — a slice whose element type is an
	// alias to an interface is still an injectable gate.
	methods, err := deepscan.NewSearcher().Usages(fixtureCriteria(t))
	require.NoError(t, err)

	for _, name := range []string{"NewProcessorBasicSlice1", "NewProcessorBasicArray1"} {
		method := findMethod(t, methods, name)
		require.Len(t, method.Gates, 1)
		assert.Equal(t, "myFetcher", method.Gates[0].Interface.Name)
	}
}

func TestCriteriaRequiresPackagePath(t *testing.T) {
	_, err := deepscan.NewCriteria()
	assert.EqualError(t, err, "criteria packagePath should be set")
}

func TestCriteriaRejectsMissingPackagePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no", "such", "dir")

	_, err := deepscan.NewCriteria(deepscan.WithPackagePath(missing))
	assert.EqualError(t, err, "failed fill optional criteria fields: "+
		"failed find root path of '"+missing+"': packagePath directory not exist")
}

func TestCriteriaFindsModuleRootUpward(t *testing.T) {
	// no analyse scope given: defaults are filled from the nearest go.mod
	// (the nested fixture module), walking up from the package dir.
	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(filepath.Join(fixtureProject(t), "internal", "operations")),
	)
	require.NoError(t, err)

	// the module name and root are private, so observe them through a scan:
	// Usages must resolve the fixture module imports (repository injections)
	// exactly as with an explicit scope.
	methods, err := deepscan.NewSearcher().Usages(criteria)
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasic1")
	require.Len(t, method.Gates, 1)
	require.Len(t, method.Gates[0].Implementations, 1)
}

func TestUsagesHonorsExcludeMatchers(t *testing.T) {
	// excludeFileMatchers filter the ANALYSE SCOPE (the recursive search for
	// injector files), not the scanned package itself: the constructor stays
	// discovered, but its implementations from the excluded file vanish.
	// (matches how the checker passes spec.ExcludeFiles into the criteria)
	criteria, err := deepscan.NewCriteria(
		deepscan.WithPackagePath(filepath.Join(fixtureProject(t), "internal", "operations")),
		deepscan.WithAnalyseScope(filepath.Join(fixtureProject(t), "internal")),
		deepscan.WithExcludedFileMatchers([]*regexp.Regexp{
			regexp.MustCompile(`di/1_basic\.go$`),
		}),
	)
	require.NoError(t, err)

	methods, err := deepscan.NewSearcher().Usages(criteria)
	require.NoError(t, err)

	method := findMethod(t, methods, "NewProcessorBasic1")
	require.Len(t, method.Gates, 1)
	assert.Empty(t, method.Gates[0].Implementations,
		"implementations must not be searched in excluded files")
}

func TestUsagesSharedSearcherInstanceIsReusable(t *testing.T) {
	// the searcher caches parsed packages between calls and guards the ctx
	// with a mutex — two sequential scans on one instance must both work.
	searcher := deepscan.NewSearcher()

	first, err := searcher.Usages(fixtureCriteria(t))
	require.NoError(t, err)

	second, err := searcher.Usages(fixtureCriteria(t))
	require.NoError(t, err)

	assert.Equal(t, methodNames(first), methodNames(second))
}
