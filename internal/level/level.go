// Package level loads and validates the development level used by the dev command.
package level

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/codec"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/directive"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/source"
)

// Development is a normalized development level and its source files.
type Development struct {
	Data  *resource.LevelData
	File  string
	Files []string
}

// CompileLevel strictly decodes a JSON LevelData document.
func CompileLevel(src string) (*resource.LevelData, error) {
	level, err := decodeStrict([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("level: parse JSON: %w", err)
	}
	return level, nil
}

// LevelBuilder constructs LevelData programmatically.
type LevelBuilder struct {
	bgmOffset float64
	entities  []resource.LevelDataEntity
}

func NewLevelBuilder() *LevelBuilder { return &LevelBuilder{} }

func (b *LevelBuilder) SetBGMOffset(offset float64) *LevelBuilder {
	b.bgmOffset = offset
	return b
}

func (b *LevelBuilder) AddEntity(name, archetype string, data []resource.LevelDataEntityData) *LevelBuilder {
	b.entities = append(b.entities, resource.LevelDataEntity{Name: name, Archetype: resource.EngineArchetypeName(archetype), Data: data})
	return b
}

func (b *LevelBuilder) Build() *resource.LevelData {
	entities := append([]resource.LevelDataEntity(nil), b.entities...)
	if entities == nil {
		entities = []resource.LevelDataEntity{}
	}
	return &resource.LevelData{BGMOffset: b.bgmOffset, Entities: entities}
}

type declaration struct {
	mode        mode.Mode
	packagePath string
	variable    string
	position    string
	file        string
	sourceFiles []string
}

var developmentModes = []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview}

// LoadDevelopment loads the shared development level declaration in all level modes.
func LoadDevelopment(patterns ...string) (*Development, error) {
	declarations := make([]*declaration, 0, len(developmentModes))
	for _, currentMode := range developmentModes {
		pkgs, err := source.LoadMode(currentMode, patterns...)
		if err != nil {
			return nil, fmt.Errorf("development level: load %s mode: %w", currentMode, err)
		}
		if len(pkgs) != 1 || pkgs[0].Name != "main" {
			return nil, fmt.Errorf("development level: %s mode expected exactly one main package, found %d", currentMode, len(pkgs))
		}
		found, err := scanDeclaration(currentMode, pkgs[0])
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, found)
	}

	present := 0
	for _, item := range declarations {
		if item != nil {
			present++
		}
	}
	if present == 0 {
		return &Development{Data: emptyLevel()}, nil
	}
	if present != len(declarations) {
		return nil, errors.New("development level: //sonolus:level must be visible in play, watch, and preview modes")
	}
	first := declarations[0]
	for _, item := range declarations[1:] {
		if item.packagePath != first.packagePath || item.variable != first.variable || item.file != first.file {
			return nil, fmt.Errorf("development level: declarations differ between %s (%s) and %s (%s)", first.mode, first.position, item.mode, item.position)
		}
	}

	data, err := os.ReadFile(first.file)
	if err != nil {
		return nil, fmt.Errorf("development level: read %s: %w", first.file, err)
	}
	level, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("development level: decode %s: %w", first.file, err)
	}
	files := append([]string(nil), first.sourceFiles...)
	files = append(files, first.file)
	sort.Strings(files)
	files = compact(files)
	return &Development{Data: level, File: first.file, Files: files}, nil
}

func scanDeclaration(currentMode mode.Mode, pkg *packages.Package) (*declaration, error) {
	var found *declaration
	var errs []error
	for _, file := range pkg.Syntax {
		for _, node := range file.Decls {
			gen, ok := node.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specNode := range gen.Specs {
				spec, ok := specNode.(*ast.ValueSpec)
				if !ok {
					continue
				}
				doc := spec.Doc
				if doc == nil {
					doc = gen.Doc
				}
				for _, dir := range directive.ParseDirectives(doc, directive.PrefixSonolus) {
					if dir.Cmd != directive.CmdLevel {
						continue
					}
					where := pkg.Fset.Position(dir.Pos).String()
					if len(dir.Args) != 0 {
						errs = append(errs, fmt.Errorf("%s: sonolus:level does not accept arguments", where))
						continue
					}
					if gen.Tok.String() != "var" || len(spec.Names) != 1 {
						errs = append(errs, fmt.Errorf("%s: sonolus:level requires exactly one package variable", where))
						continue
					}
					name := spec.Names[0]
					object, ok := pkg.TypesInfo.Defs[name].(*types.Var)
					if !ok || !isLevelFile(object.Type()) {
						errs = append(errs, fmt.Errorf("%s: sonolus:level variable must have type sonolus.LevelFile", where))
						continue
					}
					if len(spec.Values) != 0 {
						errs = append(errs, fmt.Errorf("%s: sonolus:level variable must not have an initializer", where))
						continue
					}
					files := embeddedFilesFor(pkg, pkg.Fset.Position(file.Pos()).Filename, embedPatterns(gen, spec))
					if len(files) != 1 {
						errs = append(errs, fmt.Errorf("%s: sonolus.LevelFile requires exactly one embedded file", where))
						continue
					}
					info, err := os.Stat(files[0])
					if err != nil || !info.Mode().IsRegular() {
						errs = append(errs, fmt.Errorf("%s: embedded development level must be a regular file", where))
						continue
					}
					if found != nil {
						errs = append(errs, fmt.Errorf("%s: multiple sonolus:level declarations", where))
						continue
					}
					found = &declaration{mode: currentMode, packagePath: pkg.PkgPath, variable: name.Name, position: where, file: files[0], sourceFiles: append([]string(nil), pkg.GoFiles...)}
				}
			}
		}
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}
	return found, nil
}

func isLevelFile(value types.Type) bool {
	named, ok := types.Unalias(value).(*types.Named)
	return ok && named.Obj().Name() == "LevelFile" && source.IsSonolusPkgPath(named.Obj().Pkg().Path())
}

func embedPatterns(gen *ast.GenDecl, spec *ast.ValueSpec) []string {
	doc := spec.Doc
	if doc == nil {
		doc = gen.Doc
	}
	var result []string
	if doc == nil {
		return result
	}
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

func embeddedFilesFor(pkg *packages.Package, sourceFile string, patterns []string) []string {
	if len(patterns) == 0 || sourceFile == "" {
		return nil
	}
	base := filepath.Dir(sourceFile)
	var result []string
	for _, file := range pkg.EmbedFiles {
		relative, err := filepath.Rel(base, file)
		if err != nil {
			continue
		}
		relative = filepath.ToSlash(relative)
		for _, pattern := range patterns {
			if matched, _ := path.Match(pattern, relative); matched {
				result = append(result, file)
				break
			}
		}
	}
	sort.Strings(result)
	return compact(result)
}

type rawLevel struct {
	BGMOffset *float64     `json:"bgmOffset"`
	Entities  *[]rawEntity `json:"entities"`
}

type rawEntity struct {
	Name      string             `json:"name,omitempty"`
	Archetype *string            `json:"archetype"`
	Data      *[]json.RawMessage `json:"data"`
}

type rawEntry struct {
	Name  *string  `json:"name"`
	Value *float64 `json:"value,omitempty"`
	Ref   *string  `json:"ref,omitempty"`
}

func decodeStrict(data []byte) (*resource.LevelData, error) {
	var raw rawLevel
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	if err := ensureEOF(decoder); err != nil {
		return nil, err
	}
	if raw.BGMOffset == nil || raw.Entities == nil {
		return nil, errors.New("bgmOffset and entities are required and must not be null")
	}
	result := &resource.LevelData{BGMOffset: *raw.BGMOffset, Entities: make([]resource.LevelDataEntity, len(*raw.Entities))}
	for i, entity := range *raw.Entities {
		if entity.Archetype == nil || entity.Data == nil {
			return nil, fmt.Errorf("entity %d: archetype and data are required and must not be null", i)
		}
		out := resource.LevelDataEntity{Name: entity.Name, Archetype: resource.EngineArchetypeName(*entity.Archetype), Data: make([]resource.LevelDataEntityData, len(*entity.Data))}
		for j, encoded := range *entity.Data {
			var entry rawEntry
			entryDecoder := json.NewDecoder(bytes.NewReader(encoded))
			entryDecoder.DisallowUnknownFields()
			if err := entryDecoder.Decode(&entry); err != nil {
				return nil, fmt.Errorf("entity %d data %d: %w", i, j, err)
			}
			if err := ensureEOF(entryDecoder); err != nil {
				return nil, fmt.Errorf("entity %d data %d: %w", i, j, err)
			}
			if (entry.Value == nil) == (entry.Ref == nil) {
				return nil, fmt.Errorf("entity %d data %d: exactly one of value or ref is required", i, j)
			}
			if entry.Name == nil {
				return nil, fmt.Errorf("entity %d data %d: name is required and must not be null", i, j)
			}
			if entry.Value != nil {
				out.Data[j] = resource.LevelDataEntityValueData{Name: resource.EngineArchetypeDataName(*entry.Name), Value: *entry.Value}
			} else {
				out.Data[j] = resource.LevelDataEntityRefData{Name: resource.EngineArchetypeDataName(*entry.Name), Ref: *entry.Ref}
			}
		}
		result.Entities[i] = out
	}
	return result, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func emptyLevel() *resource.LevelData {
	return &resource.LevelData{Entities: []resource.LevelDataEntity{}}
}

// Validate checks a development level against all ordinary engine modes.
func Validate(level *resource.LevelData, artifacts *compiler.Artifacts) error {
	if level == nil || artifacts == nil || artifacts.Play == nil || artifacts.Watch == nil || artifacts.Preview == nil {
		return errors.New("development level: complete play, watch, and preview artifacts are required")
	}
	imports := []map[string]map[string]bool{
		playImports(artifacts.Play), watchImports(artifacts.Watch), previewImports(artifacts.Preview),
	}
	namedEntities := make(map[string]bool)
	for index, entity := range level.Entities {
		if entity.Name == "" {
			continue
		}
		if namedEntities[entity.Name] {
			return fmt.Errorf("development level: entity %d has duplicate name %q", index, entity.Name)
		}
		namedEntities[entity.Name] = true
	}
	for index, entity := range level.Entities {
		archetype := string(entity.Archetype)
		if archetype == "" {
			return fmt.Errorf("development level: entity %d has an empty archetype", index)
		}
		for modeIndex, modeName := range []string{"play", "watch", "preview"} {
			if imports[modeIndex][archetype] == nil {
				return fmt.Errorf("development level: entity %d archetype %q is not declared in %s mode", index, archetype, modeName)
			}
		}
		seenData := make(map[string]bool)
		for dataIndex, item := range entity.Data {
			name, ref, isRef := entryIdentity(item)
			if name == "" {
				return fmt.Errorf("development level: entity %d data %d has an empty name", index, dataIndex)
			}
			if seenData[name] {
				return fmt.Errorf("development level: entity %d has duplicate data name %q", index, name)
			}
			seenData[name] = true
			for modeIndex, modeName := range []string{"play", "watch", "preview"} {
				if !imports[modeIndex][archetype][name] {
					return fmt.Errorf("development level: entity %d data %q is not imported by archetype %q in %s mode", index, name, archetype, modeName)
				}
			}
			if isRef && ref == "" {
				return fmt.Errorf("development level: entity %d data %q has an empty ref", index, name)
			}
			if isRef && !namedEntities[ref] {
				return fmt.Errorf("development level: entity %d data %q references unknown entity %q", index, name, ref)
			}
		}
	}
	return nil
}

func entryIdentity(value resource.LevelDataEntityData) (string, string, bool) {
	switch entry := value.(type) {
	case resource.LevelDataEntityValueData:
		return string(entry.Name), "", false
	case resource.LevelDataEntityRefData:
		return string(entry.Name), entry.Ref, true
	default:
		return "", "", false
	}
}

func playImports(data *resource.EnginePlayData) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(data.Archetypes))
	for _, archetype := range data.Archetypes {
		result[string(archetype.Name)] = importNames(archetype.Imports)
	}
	return result
}

func watchImports(data *resource.EngineWatchData) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(data.Archetypes))
	for _, archetype := range data.Archetypes {
		result[string(archetype.Name)] = importNames(archetype.Imports)
	}
	return result
}

func previewImports(data *resource.EnginePreviewData) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(data.Archetypes))
	for _, archetype := range data.Archetypes {
		result[string(archetype.Name)] = importNames(archetype.Imports)
	}
	return result
}

func importNames(imports []resource.EngineDataArchetypeImport) map[string]bool {
	result := make(map[string]bool, len(imports))
	for _, item := range imports {
		result[string(item.Name)] = true
	}
	return result
}

// Package returns the compressed LevelData payload served to Sonolus.
func Package(data *resource.LevelData) ([]byte, error) {
	return codec.Compress(data)
}

func compact(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
