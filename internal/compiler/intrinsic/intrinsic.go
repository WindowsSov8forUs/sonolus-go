// Package intrinsic defines the limited standard-library surface understood by
// the compiler.
package intrinsic

import (
	"fmt"
	"go/build"
	"go/types"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"golang.org/x/tools/go/packages"
)

type Kind uint8

const StandardLibraryFilterError = "this standard library package is not allowed; only embed, iter, math, and math/rand are allowed"

const (
	RuntimeFunction Kind = iota
	Constant
)

type Symbol struct {
	Package string
	Name    string
	Kind    Kind
	Runtime resource.RuntimeFunction
	Arity   int
	Prefix  []float64
	Value   float64
}

var symbols = map[string]Symbol{}

func addRuntime(pkg, name string, runtime resource.RuntimeFunction, arity int, prefix ...float64) {
	symbols[pkg+"."+name] = Symbol{Package: pkg, Name: name, Kind: RuntimeFunction, Runtime: runtime, Arity: arity, Prefix: prefix}
}

func addConstant(name string, value float64) {
	symbols["math."+name] = Symbol{Package: "math", Name: name, Kind: Constant, Value: value}
}

func init() {
	for _, entry := range []struct {
		name  string
		fn    resource.RuntimeFunction
		arity int
	}{
		{"Abs", resource.RuntimeFunctionAbs, 1}, {"Floor", resource.RuntimeFunctionFloor, 1},
		{"Ceil", resource.RuntimeFunctionCeil, 1}, {"Round", resource.RuntimeFunctionRound, 1},
		{"Trunc", resource.RuntimeFunctionTrunc, 1}, {"Log", resource.RuntimeFunctionLog, 1},
		{"Sin", resource.RuntimeFunctionSin, 1}, {"Cos", resource.RuntimeFunctionCos, 1},
		{"Tan", resource.RuntimeFunctionTan, 1}, {"Sinh", resource.RuntimeFunctionSinh, 1},
		{"Cosh", resource.RuntimeFunctionCosh, 1}, {"Tanh", resource.RuntimeFunctionTanh, 1},
		{"Asin", resource.RuntimeFunctionArcsin, 1},
		{"Acos", resource.RuntimeFunctionArccos, 1}, {"Atan", resource.RuntimeFunctionArctan, 1},
		{"Atan2", resource.RuntimeFunctionArctan2, 2}, {"Min", resource.RuntimeFunctionMin, 2},
		{"Max", resource.RuntimeFunctionMax, 2}, {"Pow", resource.RuntimeFunctionPower, 2},
		{"Mod", resource.RuntimeFunctionRem, 2},
	} {
		addRuntime("math", entry.name, entry.fn, entry.arity)
	}
	addRuntime("math/rand", "Float64", resource.RuntimeFunctionRandom, 0, 0, 1)
	addRuntime("math/rand", "Intn", resource.RuntimeFunctionRandomInteger, 1, 0)
	for name, value := range map[string]float64{
		"E": math.E, "Pi": math.Pi, "Phi": math.Phi, "Sqrt2": math.Sqrt2,
		"SqrtE": math.SqrtE, "SqrtPi": math.SqrtPi, "SqrtPhi": math.SqrtPhi,
		"Ln2": math.Ln2, "Log2E": math.Log2E, "Ln10": math.Ln10, "Log10E": math.Log10E,
	} {
		addConstant(name, value)
	}
}

func Lookup(packagePath, name string) (Symbol, bool) {
	symbol, ok := symbols[packagePath+"."+name]
	return symbol, ok
}

func LookupObject(obj types.Object) (Symbol, bool) {
	if obj == nil || obj.Pkg() == nil {
		return Symbol{}, false
	}
	return Lookup(obj.Pkg().Path(), obj.Name())
}

func IsAllowedPackage(path string) bool {
	return path == "embed" || path == "iter" || path == "math" || path == "math/rand"
}

var (
	standardDependencyOnce sync.Once
	standardDependencies   map[string]bool
)

func IsAllowedStandardDependency(path string) bool {
	standardDependencyOnce.Do(func() {
		standardDependencies = map[string]bool{}
		seen := map[string]bool{}
		var visit func(string)
		visit = func(path string) {
			if seen[path] {
				return
			}
			seen[path] = true
			pkg, err := build.Default.Import(path, "", 0)
			if err != nil {
				return
			}
			standardDependencies[path] = true
			for _, dep := range pkg.Imports {
				visit(dep)
			}
		}
		for _, path := range []string{"embed", "iter", "math", "math/rand"} {
			visit(path)
		}
	})
	return standardDependencies[path]
}

func ValidateStandardImports(roots ...*packages.Package) error {
	seen := map[string]bool{}
	var messages []string
	var visit func(*packages.Package, bool)
	visit = func(pkg *packages.Package, root bool) {
		if pkg == nil || seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true
		if !root && (pkg.Module == nil || !pkg.Module.Main) {
			return
		}
		for path, dep := range pkg.Imports {
			if dep.Module == nil {
				if !IsAllowedPackage(path) {
					for _, file := range pkg.Syntax {
						for _, spec := range file.Imports {
							if strings.Trim(spec.Path.Value, `"`) == path {
								pos := pkg.Fset.Position(spec.Pos())
								messages = append(messages, fmt.Sprintf(`%s:%d:%d: could not import %q (%s)`, pos.Filename, pos.Line, pos.Column, path, StandardLibraryFilterError))
							}
						}
					}
				}
				continue
			}
			if dep.Module.Main {
				visit(dep, false)
			}
		}
	}
	for _, root := range roots {
		visit(root, true)
	}
	if len(messages) == 0 {
		return nil
	}
	sort.Strings(messages)
	return fmt.Errorf("%s", strings.Join(messages, "\n"))
}
