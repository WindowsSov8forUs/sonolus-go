package frontend

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type Project struct {
	Configuration *resource.EngineConfiguration
	ROM           []byte
	ROMDeclared   bool
	Modes         map[mode.Mode]*ModeDeclarations
}

type ModeDeclarations struct {
	Mode          mode.Mode
	Configuration *ConfigurationDeclaration
	Resources     ModeResources
	Archetypes    []*ArchetypeDeclaration
	Globals       []*CallbackDeclaration
	ROM           *ROMDeclaration
	Streams       *StreamDeclaration
	LevelGlobals  []*LevelGlobalDeclaration
}

type LevelGlobalDeclaration struct {
	PackagePath string
	TypeName    string
	Variable    string
	Kind        string
	Storage     string
	Offset      int
	Size        int
	Fields      []*LevelGlobalFieldDeclaration
}

type LevelGlobalFieldDeclaration struct {
	GoName         string
	Kind           string
	Storage        string
	Offset         int
	Size           int
	Type           types.Type
	Object         *types.Var
	ContainerKind  string
	Capacity       int
	KeySize        int
	ElementSize    int
	RelativeOffset int
	Fields         []*LevelGlobalFieldDeclaration
	Elements       []*LevelGlobalFieldDeclaration
	ElementStride  int
}

type StreamDeclaration struct {
	PackagePath string
	TypeName    string
	Variable    string
	Size        int
	Fields      []StreamFieldDeclaration
}

type StreamFieldDeclaration struct {
	Name       string
	Type       string
	Kind       string
	ValueSlots int
	Size       int
}

type ConfigurationDeclaration struct {
	Mode        mode.Mode
	PackagePath string
	TypeName    string
	Variable    string
	Pos         token.Position
	Value       *resource.EngineConfiguration
	OptionIDs   map[*types.Var]int
	Defaults    map[*types.Var]float64
}

type ModeResources struct {
	Skin        *resource.EngineSkinData
	Effect      *resource.EngineEffectData
	Particle    *resource.EngineParticleData
	Buckets     []resource.EngineDataBucket
	Instruction *resource.EngineInstructionData
	SpriteIDs   map[string]int
	EffectIDs   map[string]int
	ParticleIDs map[string]int
	StreamSize  int
	// FieldIDs maps a resource declaration field to its scalar ID or the
	// consecutive IDs assigned to its fixed-size array elements.
	FieldIDs map[*types.Var][]int
}

type ArchetypeDeclaration struct {
	PackagePath    string
	TypeName       string
	Name           string
	Abstract       bool
	Key            float64
	HasKey         bool
	HasInput       bool
	Fields         []*FieldDeclaration
	Imports        []resource.EngineDataArchetypeImport
	Exports        []resource.EngineArchetypeDataName
	Callbacks      []*CallbackDeclaration
	CallbackOrders map[string]int
	Named          *types.Named
	BaseNamed      *types.Named
	Base           *ArchetypeDeclaration
	MRO            []*ArchetypeDeclaration
}

type FieldDeclaration struct {
	GoName         string
	SourcePath     string
	ExternalName   string
	ExternalNames  []string
	Storage        string
	Offset         int
	Size           int
	Default        float64
	Type           types.Type
	Object         *types.Var
	ReceiverOffset int
	ContainerKind  string
	Capacity       int
	KeySize        int
	ElementSize    int
}

type CallbackDeclaration struct {
	Name     string
	Order    int
	Function *types.Func
	Decl     *ast.FuncDecl
	IR       *ir.Function
}

type ROMDeclaration struct {
	Mode        mode.Mode
	PackagePath string
	Variable    string
	File        string
	Pos         token.Position
	Values      []float32
	Bytes       []byte
}
