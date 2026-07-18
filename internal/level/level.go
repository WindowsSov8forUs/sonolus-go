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
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/directive"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	compilerschema "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/schema"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

// DevelopmentLevel is one normalized development level.
type DevelopmentLevel struct {
	Name  string
	Title string
	Data  *resource.LevelData
	File  string
}

// Development is the ordered development level collection and its source files.
type Development struct {
	Levels []DevelopmentLevel
	Files  []string
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
	name        string
	position    string
	file        string
	sourceFiles []string
}

var developmentModes = []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview}

// LoadDevelopment loads the shared development level declarations in all level modes.
func LoadDevelopment(patterns ...string) (*Development, error) {
	declarations := make([][]*declaration, 0, len(developmentModes))
	for _, currentMode := range developmentModes {
		pkgs, err := source.LoadMode(currentMode, patterns...)
		if err != nil {
			return nil, fmt.Errorf("development level: load %s mode: %w", currentMode, err)
		}
		if len(pkgs) != 1 || pkgs[0].Name != "main" {
			return nil, fmt.Errorf("development level: %s mode expected exactly one main package, found %d", currentMode, len(pkgs))
		}
		found, err := scanDeclarations(currentMode, pkgs[0])
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, found)
	}

	if len(declarations[0]) == 0 && len(declarations[1]) == 0 && len(declarations[2]) == 0 {
		return &Development{Levels: []DevelopmentLevel{{Name: "dev", Title: "Dev Level", Data: emptyLevel()}}}, nil
	}
	first := declarations[0]
	for modeIndex, items := range declarations[1:] {
		if len(items) != len(first) {
			return nil, errors.New("development level: //sonolus:level declarations must all be visible in play, watch, and preview modes")
		}
		for index, item := range items {
			reference := first[index]
			if item.packagePath != reference.packagePath || item.variable != reference.variable || item.name != reference.name || item.file != reference.file {
				return nil, fmt.Errorf("development level: declarations differ between %s (%s) and %s (%s)", reference.mode, reference.position, developmentModes[modeIndex+1], item.position)
			}
		}
	}

	levels := make([]DevelopmentLevel, len(first))
	files := []string{}
	names := map[string]bool{}
	for index, item := range first {
		name := item.name
		title := splitIdentifier(item.variable)
		if name == "" {
			if len(first) != 1 {
				return nil, fmt.Errorf("%s: multiple sonolus:level declarations require a unique level name argument", item.position)
			}
			name = "dev"
			title = "Dev Level"
		}
		if names[name] {
			return nil, fmt.Errorf("%s: duplicate development level name %q", item.position, name)
		}
		names[name] = true
		data, err := os.ReadFile(item.file)
		if err != nil {
			return nil, fmt.Errorf("development level %q: read %s: %w", name, item.file, err)
		}
		level, err := decodeStrict(data)
		if err != nil {
			return nil, fmt.Errorf("development level %q: decode %s: %w", name, item.file, err)
		}
		levels[index] = DevelopmentLevel{Name: name, Title: title, Data: level, File: item.file}
		files = append(files, item.sourceFiles...)
		files = append(files, item.file)
	}
	sort.Strings(files)
	files = compact(files)
	return &Development{Levels: levels, Files: files}, nil
}

func scanDeclarations(currentMode mode.Mode, pkg *packages.Package) ([]*declaration, error) {
	var found []*declaration
	variables := map[string]bool{}
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
					if len(dir.Args) > 1 {
						errs = append(errs, fmt.Errorf("%s: sonolus:level accepts at most one level name argument", where))
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
					if variables[name.Name] {
						errs = append(errs, fmt.Errorf("%s: duplicate sonolus:level declaration for %s", where, name.Name))
						continue
					}
					variables[name.Name] = true
					levelName := ""
					if len(dir.Args) == 1 {
						levelName = dir.Args[0]
					}
					found = append(found, &declaration{mode: currentMode, packagePath: pkg.PkgPath, variable: name.Name, name: levelName, position: where, file: files[0], sourceFiles: append([]string(nil), pkg.GoFiles...)})
				}
			}
		}
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].variable < found[j].variable })
	return found, nil
}

func splitIdentifier(value string) string {
	var result strings.Builder
	for index, current := range value {
		if index > 0 && current >= 'A' && current <= 'Z' {
			previous := value[index-1]
			if previous >= 'a' && previous <= 'z' {
				result.WriteByte(' ')
			}
		}
		result.WriteRune(current)
	}
	return result.String()
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
	contract := compilerschema.Build(playSchema(artifacts.Play), watchSchema(artifacts.Watch), previewSchema(artifacts.Preview))
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
		for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview} {
			if !contract.HasArchetype(currentMode, archetype) {
				return fmt.Errorf("development level: entity %d archetype %q is not declared in %s mode", index, archetype, currentMode)
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
			for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview} {
				if !contract.IsImported(currentMode, archetype, name) {
					return fmt.Errorf("development level: entity %d data %q is not imported by archetype %q in %s mode", index, name, archetype, currentMode)
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

func playSchema(data *resource.EnginePlayData) []compilerschema.ModeArchetype {
	result := make([]compilerschema.ModeArchetype, len(data.Archetypes))
	for i, archetype := range data.Archetypes {
		result[i] = compilerschema.ModeArchetype{Name: string(archetype.Name), Imports: importNames(archetype.Imports), Exports: exportNames(archetype.Exports)}
	}
	return result
}

func watchSchema(data *resource.EngineWatchData) []compilerschema.ModeArchetype {
	result := make([]compilerschema.ModeArchetype, len(data.Archetypes))
	for i, archetype := range data.Archetypes {
		result[i] = compilerschema.ModeArchetype{Name: string(archetype.Name), Imports: importNames(archetype.Imports)}
	}
	return result
}

func previewSchema(data *resource.EnginePreviewData) []compilerschema.ModeArchetype {
	result := make([]compilerschema.ModeArchetype, len(data.Archetypes))
	for i, archetype := range data.Archetypes {
		result[i] = compilerschema.ModeArchetype{Name: string(archetype.Name), Imports: importNames(archetype.Imports)}
	}
	return result
}

func importNames(imports []resource.EngineDataArchetypeImport) []string {
	result := make([]string, len(imports))
	for i, item := range imports {
		result[i] = string(item.Name)
	}
	return result
}

func exportNames(exports []resource.EngineArchetypeDataName) []string {
	result := make([]string, len(exports))
	for i, item := range exports {
		result[i] = string(item)
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
