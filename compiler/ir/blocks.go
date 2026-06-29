package ir

// Code generated from sonolus.py backend/blocks.py. DO NOT EDIT by hand.
// Regenerate via compiler/ir/optimize/testdata/genblocks (see _blocks.json).

// Mode is a Sonolus engine mode; each has its own memory block layout.
type Mode int

const (
	ModePlay Mode = iota
	ModeWatch
	ModePreview
	ModeTutorial
)

type blockInfo struct {
	name     string
	writable map[string]bool
}

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

var runtimeConstantBlocks = set("ArchetypeLife", "EngineRom", "LevelBucket", "LevelData", "LevelLife", "LevelOption", "LevelScore", "PreviewData", "PreviewOption", "RuntimeCanvas", "RuntimeEnvironment", "RuntimeUI", "RuntimeUIConfiguration", "TutorialData")

var blockTables = map[Mode]map[int]blockInfo{
	ModePlay: {
		1000:  {"RuntimeEnvironment", set("preprocess")},
		1001:  {"RuntimeUpdate", nil},
		1002:  {"RuntimeTouchArray", nil},
		1003:  {"RuntimeSkinTransform", set("preprocess", "touch", "updateSequential")},
		1004:  {"RuntimeParticleTransform", set("preprocess", "touch", "updateSequential")},
		1005:  {"RuntimeBackground", set("preprocess", "touch", "updateSequential")},
		1006:  {"RuntimeUI", set("preprocess")},
		1007:  {"RuntimeUIConfiguration", set("preprocess")},
		2000:  {"LevelMemory", set("preprocess", "touch", "updateSequential")},
		2001:  {"LevelData", set("preprocess")},
		2002:  {"LevelOption", nil},
		2003:  {"LevelBucket", set("preprocess")},
		2004:  {"LevelScore", set("preprocess")},
		2005:  {"LevelLife", set("preprocess")},
		3000:  {"EngineRom", nil},
		4000:  {"EntityMemory", set("initialize", "preprocess", "shouldSpawn", "spawnOrder", "terminate", "touch", "updateParallel", "updateSequential")},
		4001:  {"EntityData", set("preprocess")},
		4002:  {"EntitySharedMemory", set("preprocess", "touch", "updateSequential")},
		4003:  {"EntityInfo", nil},
		4004:  {"EntityDespawn", set("initialize", "preprocess", "shouldSpawn", "spawnOrder", "terminate", "touch", "updateParallel", "updateSequential")},
		4005:  {"EntityInput", set("initialize", "preprocess", "shouldSpawn", "spawnOrder", "terminate", "touch", "updateParallel", "updateSequential")},
		4006:  {"EntityScore", set("preprocess")},
		4007:  {"EntityLife", set("preprocess")},
		4101:  {"EntityDataArray", set("preprocess")},
		4102:  {"EntitySharedMemoryArray", set("preprocess", "touch", "updateSequential")},
		4103:  {"EntityInfoArray", nil},
		4106:  {"EntityScoreArray", set("preprocess")},
		4107:  {"EntityLifeArray", set("preprocess")},
		5000:  {"ArchetypeLife", set("preprocess")},
		5001:  {"ArchetypeScore", set("preprocess")},
		10000: {"TemporaryMemory", set("initialize", "preprocess", "shouldSpawn", "spawnOrder", "terminate", "touch", "updateParallel", "updateSequential")},
	},
	ModeWatch: {
		1000:  {"RuntimeEnvironment", set("preprocess")},
		1001:  {"RuntimeUpdate", nil},
		1002:  {"RuntimeSkinTransform", set("preprocess", "updateSequential")},
		1003:  {"RuntimeParticleTransform", set("preprocess", "updateSequential")},
		1004:  {"RuntimeBackground", set("preprocess", "updateSequential")},
		1005:  {"RuntimeUI", set("preprocess")},
		1006:  {"RuntimeUIConfiguration", set("preprocess")},
		2000:  {"LevelMemory", set("preprocess", "updateSequential")},
		2001:  {"LevelData", set("preprocess")},
		2002:  {"LevelOption", nil},
		2003:  {"LevelBucket", set("preprocess")},
		2004:  {"LevelScore", set("preprocess")},
		2005:  {"LevelLife", set("preprocess")},
		3000:  {"EngineRom", nil},
		4000:  {"EntityMemory", set("despawnTime", "initialize", "preprocess", "spawnTime", "terminate", "updateParallel", "updateSequential")},
		4001:  {"EntityData", set("preprocess")},
		4002:  {"EntitySharedMemory", set("preprocess", "updateSequential")},
		4003:  {"EntityInfo", nil},
		4004:  {"EntityInput", set("preprocess")},
		4005:  {"EntityScore", set("preprocess")},
		4006:  {"EntityLife", set("preprocess")},
		4101:  {"EntityDataArray", set("preprocess")},
		4102:  {"EntitySharedMemoryArray", set("preprocess", "updateSequential")},
		4103:  {"EntityInfoArray", nil},
		4105:  {"EntityScoreArray", set("preprocess")},
		4106:  {"EntityLifeArray", set("preprocess")},
		5000:  {"ArchetypeLife", set("preprocess")},
		5001:  {"ArchetypeScore", set("preprocess")},
		10000: {"TemporaryMemory", set("despawnTime", "initialize", "preprocess", "spawnTime", "terminate", "updateParallel", "updateSequential", "updateSpawn")},
	},
	ModePreview: {
		1000:  {"RuntimeEnvironment", set("preprocess")},
		1001:  {"RuntimeCanvas", set("preprocess")},
		1002:  {"RuntimeSkinTransform", set("preprocess")},
		1003:  {"RuntimeUI", set("preprocess")},
		1004:  {"RuntimeUIConfiguration", set("preprocess")},
		2000:  {"PreviewData", set("preprocess")},
		2001:  {"PreviewOption", nil},
		3000:  {"EngineRom", nil},
		4000:  {"EntityData", set("preprocess")},
		4001:  {"EntitySharedMemory", set("preprocess")},
		4002:  {"EntityInfo", nil},
		4100:  {"EntityDataArray", set("preprocess")},
		4101:  {"EntitySharedMemoryArray", set("preprocess")},
		4102:  {"EntityInfoArray", nil},
		10000: {"TemporaryMemory", set("preprocess", "render")},
	},
	ModeTutorial: {
		1000:  {"RuntimeEnvironment", set("preprocess")},
		1001:  {"RuntimeUpdate", nil},
		1002:  {"RuntimeSkinTransform", set("navigate", "preprocess", "update")},
		1003:  {"RuntimeParticleTransform", set("navigate", "preprocess", "update")},
		1004:  {"RuntimeBackground", set("navigate", "preprocess", "update")},
		1005:  {"RuntimeUI", set("preprocess")},
		1006:  {"RuntimeUIConfiguration", set("preprocess")},
		2000:  {"TutorialMemory", set("navigate", "preprocess", "update")},
		2001:  {"TutorialData", set("preprocess")},
		2002:  {"TutorialInstruction", set("navigate", "preprocess", "update")},
		3000:  {"EngineRom", nil},
		10000: {"TemporaryMemory", set("navigate", "preprocess", "update")},
	},
}

// Canonical block ID constants referenced by all packages.
// Values must match the blockTables above.
const (
	BlockEntityMemory  = 4000
	BlockEntityData    = 4001
	BlockEntityShared  = 4002
	BlockEntityInfo    = 4003
	BlockEntityDespawn = 4004
	BlockEntityInput   = 4005
	BlockEntityScore   = 4006
	BlockEntityLife    = 4007
	BlockTempMemory    = 10000
)

// BlockSet answers block read/write questions for a given mode. It satisfies
// the optimizer's BlockOracle interface structurally.
type BlockSet struct{ mode Mode }

// Blocks returns the block model for a mode.
func Blocks(mode Mode) BlockSet { return BlockSet{mode} }

func (b BlockSet) info(id int) (blockInfo, bool) {
	m, ok := blockTables[b.mode]
	if !ok {
		return blockInfo{}, false
	}
	bi, ok := m[id]
	return bi, ok
}

// Writable reports whether the block may be written in the given callback.
// Unknown blocks are assumed writable (conservative: not safe to inline reads).
func (b BlockSet) Writable(id int, callback string) bool {
	bi, ok := b.info(id)
	if !ok {
		return true
	}
	return bi.writable[callback]
}

// RuntimeConstant reports whether the block holds runtime-constant data.
func (b BlockSet) RuntimeConstant(id int) bool {
	bi, ok := b.info(id)
	if !ok {
		return false
	}
	return runtimeConstantBlocks[bi.name]
}
