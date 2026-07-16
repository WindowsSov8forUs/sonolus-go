package frontend

import (
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/types"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

func embedPatterns(gen *ast.GenDecl, spec *ast.ValueSpec) []string {
	doc := spec.Doc
	if doc == nil {
		doc = gen.Doc
	}
	if doc == nil {
		return nil
	}
	var result []string
	for _, comment := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if !strings.HasPrefix(text, "go:embed") {
			continue
		}
		for _, pattern := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(text, "go:embed"))) {
			if unquoted, err := strconv.Unquote(pattern); err == nil {
				pattern = unquoted
			}
			result = append(result, pattern)
		}
	}
	return result
}

func embeddedFilesFor(pkg *packages.Package, patterns []string) []string {
	if len(patterns) == 0 || len(pkg.GoFiles) == 0 {
		return nil
	}
	base := filepath.Dir(pkg.GoFiles[0])
	seen := map[string]bool{}
	var result []string
	for _, file := range pkg.EmbedFiles {
		rel, err := filepath.Rel(base, file)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		for _, pattern := range patterns {
			matched, _ := path.Match(pattern, rel)
			if matched && !seen[file] {
				seen[file] = true
				result = append(result, file)
			}
		}
	}
	sort.Strings(result)
	return result
}

func packageROM(pkg *packages.Package, tracer *source.ASTTracer) (*ROMDeclaration, []error) {
	var found *ROMDeclaration
	var errs []error
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok.String() != "var" {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, name := range vs.Names {
					obj, ok := pkg.TypesInfo.Defs[name].(*types.Var)
					if !ok {
						continue
					}
					id := typeID(obj.Type())
					if id != rootID("ROMValues") && id != rootID("ROMFile") {
						continue
					}
					if found != nil {
						errs = append(errs, fmt.Errorf("multiple ROM declarations"))
						continue
					}
					rom := &ROMDeclaration{PackagePath: pkg.PkgPath, Variable: name.Name, Pos: pkg.Fset.Position(name.Pos())}
					if id == rootID("ROMValues") {
						binding, err := tracer.EvalObject(obj)
						if err != nil {
							errs = append(errs, fmt.Errorf("%s: ROMValues must be statically evaluable: %w", name.Name, err))
							continue
						}
						if err := pureStaticError(binding.Value, "ROMValues"); err != nil {
							errs = append(errs, err)
							continue
						}
						elements, ok := staticElements(binding.Value)
						if !ok {
							errs = append(errs, fmt.Errorf("%s: ROMValues must be a static sequence", name.Name))
							continue
						}
						for _, element := range elements {
							f, valid := staticNumber(element)
							if !valid {
								errs = append(errs, fmt.Errorf("%s: ROM value must be numeric", name.Name))
								continue
							}
							rom.Values = append(rom.Values, float32(f))
						}
						rom.Bytes = make([]byte, 4*len(rom.Values))
						for i, value := range rom.Values {
							binary.LittleEndian.PutUint32(rom.Bytes[4*i:], math.Float32bits(value))
						}
					} else {
						files := embeddedFilesFor(pkg, embedPatterns(gen, vs))
						if len(files) != 1 {
							errs = append(errs, fmt.Errorf("%s: ROMFile requires exactly one embedded file", name.Name))
							continue
						}
						data, err := os.ReadFile(files[0])
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if len(data)%4 != 0 {
							errs = append(errs, fmt.Errorf("%s: ROM file length must be divisible by 4", name.Name))
							continue
						}
						rom.File = files[0]
						rom.Bytes = data
						for i := 0; i < len(data); i += 4 {
							rom.Values = append(rom.Values, math.Float32frombits(binary.LittleEndian.Uint32(data[i:])))
						}
					}
					found = rom
				}
			}
		}
	}
	return found, errs
}
