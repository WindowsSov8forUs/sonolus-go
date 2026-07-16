package frontend

import (
	"bytes"
	"fmt"
	"go/types"
	"reflect"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

type RuntimeChecks int

const (
	RuntimeChecksNone RuntimeChecks = iota
	RuntimeChecksTerminate
	RuntimeChecksNotify
)

type Options struct {
	RuntimeChecks RuntimeChecks
}

func parsePackage(pkg *packages.Package, m mode.Mode, options Options) (*ModeDeclarations, error) {
	out := &ModeDeclarations{Mode: m, Resources: ModeResources{SpriteIDs: map[string]int{}, EffectIDs: map[string]int{}, ParticleIDs: map[string]int{}, FieldIDs: map[*types.Var][]int{}}}
	var errs []error
	names := map[string]bool{}
	resources := map[string]bool{}
	configurationFound := false
	streamFound := false
	globalFound := false
	levelGlobalOffsets := map[string]int{}
	levelGlobalFields := map[*types.Var]*LevelGlobalFieldDeclaration{}
	type globalPackage struct {
		pkg       *packages.Package
		hasMarker bool
	}
	var globalPackages []globalPackage
	userPackages := collectPackages(pkg)
	tracer := source.NewASTTracer(pkg)
	packagesByTypes := make(map[*types.Package]*packages.Package, len(userPackages))
	for _, p := range userPackages {
		packagesByTypes[p.Types] = p
	}
	var allResources []resourceDeclarationSpec
	for _, p := range userPackages {
		specs, resourceErrs := resourceDeclarations(p)
		errs = append(errs, resourceErrs...)
		allResources = append(allResources, specs...)
	}
	priority := map[string]int{"Skin": 0, "Effect": 1, "Particle": 2, "Instruction": 3, "InstructionIcon": 4, "Buckets": 5}
	sort.SliceStable(allResources, func(i, j int) bool {
		if priority[allResources[i].kind] != priority[allResources[j].kind] {
			return priority[allResources[i].kind] < priority[allResources[j].kind]
		}
		return allResources[i].pos < allResources[j].pos
	})
	for _, spec := range allResources {
		if resources[spec.kind] {
			errs = append(errs, fmt.Errorf("%s: duplicate %s resource", spec.pos, strings.ToLower(spec.kind)))
			continue
		}
		allowed := spec.kind == "Skin" ||
			((spec.kind == "Effect" || spec.kind == "Particle") && m != mode.ModePreview) ||
			(spec.kind == "Buckets" && (m == mode.ModePlay || m == mode.ModeWatch)) ||
			((spec.kind == "Instruction" || spec.kind == "InstructionIcon") && m == mode.ModeTutorial)
		if !allowed {
			errs = append(errs, fmt.Errorf("%s: %s resources are not available in %s mode", spec.pos, strings.ToLower(spec.kind), m))
			continue
		}
		resources[spec.kind] = true
		switch spec.kind {
		case "Skin":
			renderMode, renderErr := skinRenderMode(spec, tracer)
			if renderErr != nil {
				errs = append(errs, renderErr)
				continue
			}
			out.Resources.Skin = &resource.EngineSkinData{RenderMode: renderMode}
		case "Effect":
			out.Resources.Effect = &resource.EngineEffectData{}
		case "Particle":
			out.Resources.Particle = &resource.EngineParticleData{}
		case "Instruction", "InstructionIcon":
			if out.Resources.Instruction == nil {
				out.Resources.Instruction = &resource.EngineInstructionData{}
			}
		}
		errs = append(errs, addResource(out, spec, tracer)...)
	}
	for _, p := range userPackages {
		rom, romErrs := packageROM(p, tracer)
		errs = append(errs, romErrs...)
		if rom != nil {
			rom.Mode = m
			if out.ROM != nil {
				errs = append(errs, fmt.Errorf("multiple ROM declarations"))
			} else {
				out.ROM = rom
			}
		}
		hasGlobals := false
		for _, named := range packageNamedTypes(p) {
			primary, ok, markerErrs := primaryDeclarationMarker(named)
			errs = append(errs, markerErrs...)
			if !ok {
				continue
			}
			id, marker := primary.id, primary.tag
			if id != rootID("Configuration") && id != rootID("StreamResource") && id != rootID("LevelMemoryResource") && id != rootID("LevelDataResource") && id != markerID(m, "Archetype") && id != markerID(m, "GlobalCallbacks") {
				continue
			}
			if id == rootID("LevelMemoryResource") || id == rootID("LevelDataResource") {
				kind := "memory"
				if id == rootID("LevelDataResource") {
					kind = "data"
				}
				storage, allowed := levelGlobalStorage(kind, m)
				if !allowed {
					errs = append(errs, fmt.Errorf("%s: level %s globals are not available in %s mode", named.Obj().Name(), kind, m))
					continue
				}
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: level %s marker requires exactly one singleton variable", named.Obj().Name(), kind))
					continue
				}
				if _, ok := variableInitializer(p, vars[0]); !ok {
					errs = append(errs, fmt.Errorf("%s: level %s singleton must have one explicit initializer", vars[0].Name(), kind))
					continue
				}
				declaration, globalErrs := parseLevelGlobal(p, named, vars[0], tracer, kind, storage, levelGlobalOffsets[storage])
				errs = append(errs, globalErrs...)
				levelGlobalOffsets[storage] += declaration.Size
				out.LevelGlobals = append(out.LevelGlobals, declaration)
				for _, field := range declaration.Fields {
					levelGlobalFields[field.Object] = field
				}
				continue
			}
			if id == rootID("StreamResource") {
				if m != mode.ModePlay && m != mode.ModeWatch {
					errs = append(errs, fmt.Errorf("%s: stream resources are only available in play and watch modes", named.Obj().Name()))
					continue
				}
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: stream resource marker requires exactly one singleton variable", named.Obj().Name()))
					continue
				}
				initializer, ok := variableInitializer(p, vars[0])
				if !ok || !validStreamInitializer(p, named, initializer) {
					errs = append(errs, fmt.Errorf("%s: stream resource singleton must have one explicit initializer", vars[0].Name()))
					continue
				}
				if streamFound {
					errs = append(errs, fmt.Errorf("duplicate stream resource declaration"))
					continue
				}
				streamFound = true
				streams, ids, streamErrs := parseStreams(named, vars[0])
				errs = append(errs, streamErrs...)
				out.Streams = streams
				out.Resources.StreamSize = streams.Size
				for field, values := range ids {
					out.Resources.FieldIDs[field] = values
				}
				continue
			}
			if id == markerID(m, "Archetype") {
				a, parseErrs := parseArchetype(packagesByTypes, p, named, m, marker)
				errs = append(errs, parseErrs...)
				if names[a.Name] {
					errs = append(errs, fmt.Errorf("duplicate archetype %q", a.Name))
				} else {
					names[a.Name] = true
					out.Archetypes = append(out.Archetypes, a)
				}
				continue
			}
			if id == rootID("Configuration") {
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: configuration marker requires exactly one singleton variable", named.Obj().Name()))
					continue
				}
				if configurationFound {
					errs = append(errs, fmt.Errorf("duplicate configuration declaration"))
					continue
				}
				configurationFound = true
				cfg, optionIDs, defaults, cfgErrs := parseConfiguration(named, vars[0], tracer)
				errs = append(errs, cfgErrs...)
				out.Configuration = &ConfigurationDeclaration{
					Mode:        m,
					PackagePath: p.PkgPath,
					TypeName:    named.Obj().Name(),
					Variable:    vars[0].Name(),
					Pos:         p.Fset.Position(vars[0].Pos()),
					Value:       cfg,
					OptionIDs:   optionIDs,
					Defaults:    defaults,
				}
				continue
			}
			if id == markerID(m, "GlobalCallbacks") {
				vars := markerVariables(p, named)
				if len(vars) != 1 {
					errs = append(errs, fmt.Errorf("%s: global callback marker requires exactly one singleton variable", named.Obj().Name()))
				} else if globalFound {
					errs = append(errs, fmt.Errorf("duplicate global callback declaration"))
				} else {
					globalFound = true
					hasGlobals = true
				}
				continue
			}
		}
		globalPackages = append(globalPackages, globalPackage{pkg: p, hasMarker: hasGlobals})
	}
	for _, candidate := range globalPackages {
		globals, globalErrs := globalCallbacks(packagesByTypes, candidate.pkg, &out.Resources, out.Configuration, levelGlobalFields, m, candidate.hasMarker, options.RuntimeChecks)
		errs = append(errs, globalErrs...)
		out.Globals = append(out.Globals, globals...)
	}
	sort.Slice(out.Archetypes, func(i, j int) bool {
		if out.Archetypes[i].Name == out.Archetypes[j].Name {
			return out.Archetypes[i].PackagePath < out.Archetypes[j].PackagePath
		}
		return out.Archetypes[i].Name < out.Archetypes[j].Name
	})
	errs = append(errs, resolveArchetypeInheritance(out.Archetypes)...)
	archetypes := make(map[*types.Named]archetypeBinding, len(out.Archetypes))
	nextArchetypeID := 0
	for _, declaration := range out.Archetypes {
		id := -1
		if !declaration.Abstract {
			id = nextArchetypeID
			nextArchetypeID++
		}
		archetypes[declaration.Named] = archetypeBinding{id: id, declaration: declaration}
	}
	for _, declaration := range out.Archetypes {
		if declaration.Abstract {
			continue
		}
		owner := packagesByTypes[declaration.Named.Obj().Pkg()]
		if owner == nil {
			errs = append(errs, fmt.Errorf("%s: archetype source package is unavailable", declaration.Name))
			continue
		}
		errs = append(errs, lowerArchetypeCallbacks(packagesByTypes, owner, declaration, &out.Resources, out.Configuration, levelGlobalFields, archetypes, m, options.RuntimeChecks)...)
	}
	concrete := out.Archetypes[:0]
	for _, declaration := range out.Archetypes {
		if !declaration.Abstract {
			concrete = append(concrete, declaration)
		}
	}
	out.Archetypes = concrete
	if len(errs) > 0 {
		messages := make([]string, len(errs))
		for i, err := range errs {
			messages[i] = err.Error()
		}
		sort.Strings(messages)
		return nil, fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return out, nil
}

var orderedModes = []mode.Mode{
	mode.ModePlay,
	mode.ModeWatch,
	mode.ModePreview,
	mode.ModeTutorial,
}

type Parser struct {
	mu      sync.RWMutex
	modes   map[mode.Mode]*ModeDeclarations
	options Options
}

// NewParser creates an empty frontend parser. Call Parse once for each loaded mode.
func NewParser(options ...Options) *Parser {
	var value Options
	if len(options) != 0 {
		value = options[0]
	}
	return &Parser{modes: make(map[mode.Mode]*ModeDeclarations, len(orderedModes)), options: value}
}

// Parse converts one already loaded mode package into frontend declarations.
func (p *Parser) Parse(m mode.Mode, pkg *packages.Package) error {
	if !m.Valid() {
		return fmt.Errorf("invalid Sonolus mode %q; expected play, watch, preview, or tutorial", m)
	}
	if pkg == nil {
		return fmt.Errorf("%s mode package is nil", m)
	}
	p.mu.RLock()
	_, exists := p.modes[m]
	p.mu.RUnlock()
	if exists {
		return fmt.Errorf("%s mode has already been parsed", m)
	}
	decl, err := parsePackage(pkg, m, p.options)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.modes[m]; exists {
		return fmt.Errorf("%s mode has already been parsed", m)
	}
	p.modes[m] = decl
	return nil
}

// GetProject validates shared declarations and merges all parsed modes.
func (p *Parser) GetProject() (*Project, error) {
	p.mu.RLock()
	declarations := make([]*ModeDeclarations, 0, len(p.modes))
	for _, m := range orderedModes {
		if decl := p.modes[m]; decl != nil {
			declarations = append(declarations, decl)
		}
	}
	p.mu.RUnlock()
	if len(declarations) == 0 {
		return nil, fmt.Errorf("no Sonolus modes have been parsed")
	}
	if err := validateShared(declarations); err != nil {
		return nil, err
	}
	project := &Project{
		Configuration: emptyConfiguration(),
		Modes:         make(map[mode.Mode]*ModeDeclarations, len(declarations)),
	}
	for i, decl := range declarations {
		project.Modes[decl.Mode] = decl
		if i == 0 {
			if decl.Configuration != nil {
				project.Configuration = decl.Configuration.Value
			}
			if decl.ROM != nil {
				project.ROM = append([]byte(nil), decl.ROM.Bytes...)
			}
		}
	}
	return project, nil
}

func validateShared(declarations []*ModeDeclarations) error {
	if len(declarations) < 2 {
		return nil
	}
	base := declarations[0]
	for _, current := range declarations[1:] {
		if err := compareConfigurations(base.Configuration, current.Configuration); err != nil {
			return err
		}
		if err := compareROM(base.ROM, current.ROM); err != nil {
			return err
		}
	}
	var playStreams, watchStreams *StreamDeclaration
	var hasPlay, hasWatch bool
	for _, declaration := range declarations {
		switch declaration.Mode {
		case mode.ModePlay:
			hasPlay = true
			playStreams = declaration.Streams
		case mode.ModeWatch:
			hasWatch = true
			watchStreams = declaration.Streams
		}
	}
	if hasPlay && hasWatch && (playStreams != nil || watchStreams != nil) {
		if err := compareStreams(playStreams, watchStreams); err != nil {
			return err
		}
	}
	return nil
}

func emptyConfiguration() *resource.EngineConfiguration {
	return &resource.EngineConfiguration{Options: []resource.EngineConfigurationOption{}, UI: defaultConfigurationUI()}
}

func configurationValue(decl *ConfigurationDeclaration) *resource.EngineConfiguration {
	if decl == nil || decl.Value == nil {
		return emptyConfiguration()
	}
	return decl.Value
}

func declarationSource(m mode.Mode, pos string) string {
	if pos == "" {
		return fmt.Sprintf("%s (<none>)", m)
	}
	return fmt.Sprintf("%s (%s)", m, pos)
}

func configurationSource(decl *ConfigurationDeclaration) string {
	if decl == nil {
		return "<none>"
	}
	return decl.Pos.String()
}

func compareConfigurations(a, b *ConfigurationDeclaration) error {
	path, left, right, different := firstDifference(reflect.ValueOf(configurationValue(a)), reflect.ValueOf(configurationValue(b)), "configuration")
	if !different {
		return nil
	}
	var leftMode, rightMode mode.Mode
	if a != nil {
		leftMode = a.Mode
	}
	if b != nil {
		rightMode = b.Mode
	}
	return fmt.Errorf("configuration differs between %s and %s: %s: %v != %v",
		declarationSource(leftMode, configurationSource(a)),
		declarationSource(rightMode, configurationSource(b)), path, left, right)
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "" || name == "-" {
		return field.Name
	}
	return name
}

func firstDifference(a, b reflect.Value, path string) (string, any, any, bool) {
	for a.IsValid() && (a.Kind() == reflect.Interface || a.Kind() == reflect.Pointer) {
		if a.IsNil() {
			break
		}
		a = a.Elem()
	}
	for b.IsValid() && (b.Kind() == reflect.Interface || b.Kind() == reflect.Pointer) {
		if b.IsNil() {
			break
		}
		b = b.Elem()
	}
	if !a.IsValid() || !b.IsValid() || a.Type() != b.Type() {
		return path, reflectedValue(a), reflectedValue(b), true
	}
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			fieldPath := path + "." + jsonFieldName(a.Type().Field(i))
			if p, left, right, ok := firstDifference(a.Field(i), b.Field(i), fieldPath); ok {
				return p, left, right, true
			}
		}
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return path + ".length", a.Len(), b.Len(), true
		}
		for i := 0; i < a.Len(); i++ {
			if p, left, right, ok := firstDifference(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); ok {
				return p, left, right, true
			}
		}
	case reflect.Map:
		if a.Len() != b.Len() {
			return path + ".length", a.Len(), b.Len(), true
		}
		keys := a.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		for _, key := range keys {
			bv := b.MapIndex(key)
			if !bv.IsValid() {
				return fmt.Sprintf("%s[%v]", path, key.Interface()), reflectedValue(a.MapIndex(key)), nil, true
			}
			if p, left, right, ok := firstDifference(a.MapIndex(key), bv, fmt.Sprintf("%s[%v]", path, key.Interface())); ok {
				return p, left, right, true
			}
		}
	default:
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			return path, a.Interface(), b.Interface(), true
		}
	}
	return "", nil, nil, false
}

func reflectedValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	return value.Interface()
}

func romSource(decl *ROMDeclaration) string {
	if decl == nil {
		return "<none>"
	}
	return decl.Pos.String()
}

func compareROM(a, b *ROMDeclaration) error {
	var left, right []byte
	var leftMode, rightMode mode.Mode
	if a != nil {
		left, leftMode = a.Bytes, a.Mode
	}
	if b != nil {
		right, rightMode = b.Bytes, b.Mode
	}
	if bytes.Equal(left, right) {
		return nil
	}
	offset := 0
	for offset < len(left) && offset < len(right) && left[offset] == right[offset] {
		offset++
	}
	return fmt.Errorf("ROM differs between %s and %s: first differing byte at offset %d (length %d != %d)",
		declarationSource(leftMode, romSource(a)), declarationSource(rightMode, romSource(b)), offset, len(left), len(right))
}
