package frontend

import (
	"go/ast"
	"go/types"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/intrinsic"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

type EngineDeclarations struct {
	Mode          mode.Mode
	Configuration *resource.EngineConfiguration
	Resources     ModeResources
	Archetypes    []*ArchetypeDeclaration
	Globals       []*CallbackDeclaration
	ROM           *ROMDeclaration
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
	// FieldIDs maps a resource declaration field to its scalar ID or the
	// consecutive IDs assigned to its fixed-size array elements.
	FieldIDs map[*types.Var][]int
}

type ArchetypeDeclaration struct {
	PackagePath string
	TypeName    string
	Name        string
	HasInput    bool
	Fields      []*FieldDeclaration
	Imports     []resource.EngineDataArchetypeImport
	Exports     []resource.EngineArchetypeDataName
	Callbacks   []*CallbackDeclaration
	Named       *types.Named
}

type FieldDeclaration struct {
	GoName       string
	ExternalName string
	Storage      string
	Offset       int
	Size         int
	Default      float64
	Type         types.Type
}

type CallbackDeclaration struct {
	Name       string
	Order      int
	Function   *types.Func
	Decl       *ast.FuncDecl
	Intrinsics []IntrinsicReference
}

type IntrinsicReference struct {
	Symbol intrinsic.Symbol
	API    *catalog.Symbol
	Object types.Object
}

type ROMDeclaration struct {
	PackagePath string
	Variable    string
	File        string
	Values      []float32
	Bytes       []byte
}
