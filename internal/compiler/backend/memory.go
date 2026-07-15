package backend

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

const (
	engineROMBlock       = 3000
	temporaryMemoryBlock = 10000
)

var memoryBlocks = map[mode.Mode]map[string]int{
	mode.ModePlay: {
		"RuntimeEnvironment": 1000, "RuntimeUpdate": 1001, "RuntimeTouch": 1002,
		"RuntimeTransform": 1003, "SkinTransform": 1003, "ParticleTransform": 1004,
		"RuntimeBackground": 1005, "RuntimeUI": 1006, "RuntimeUIConfiguration": 1007,
		"LevelMemory": 2000, "LevelData": 2001, "LevelOption": 2002, "LevelBucket": 2003,
		"LevelScore": 2004, "LevelLife": 2005, "EngineRom": 3000,
		"memory": 4000, "data": 4001, "shared": 4002,
		"CurrentEntityInfo": 4003, "CurrentEntityDespawn": 4004, "CurrentInputResult": 4005,
		"EntityDataArray": 4101, "EntitySharedMemoryArray": 4102, "EntityInfo": 4103,
		"ArchetypeLife": 5000, "ArchetypeScore": 5001,
	},
	mode.ModeWatch: {
		"RuntimeEnvironment": 1000, "RuntimeUpdate": 1001,
		"RuntimeTransform": 1002, "SkinTransform": 1002, "ParticleTransform": 1003,
		"RuntimeBackground": 1004, "RuntimeUI": 1005, "RuntimeUIConfiguration": 1006,
		"LevelMemory": 2000, "LevelData": 2001, "LevelOption": 2002, "LevelBucket": 2003,
		"LevelScore": 2004, "LevelLife": 2005, "EngineRom": 3000,
		"memory": 4000, "data": 4001, "shared": 4002,
		"CurrentEntityInfo": 4003, "CurrentInputResult": 4004,
		"EntityDataArray": 4101, "EntitySharedMemoryArray": 4102, "EntityInfo": 4103,
		"ArchetypeLife": 5000, "ArchetypeScore": 5001,
	},
	mode.ModePreview: {
		"RuntimeEnvironment": 1000, "RuntimeCanvas": 1001,
		"RuntimeTransform": 1002, "SkinTransform": 1002,
		"RuntimeUI": 1003, "RuntimeUIConfiguration": 1004,
		"PreviewData": 2000, "PreviewOption": 2001, "EngineRom": 3000,
		"data": 4000, "shared": 4001, "CurrentEntityInfo": 4002,
		"EntityDataArray": 4100, "EntitySharedMemoryArray": 4101, "EntityInfo": 4102,
	},
	mode.ModeTutorial: {
		"RuntimeEnvironment": 1000, "RuntimeUpdate": 1001,
		"RuntimeTransform": 1002, "SkinTransform": 1002, "ParticleTransform": 1003,
		"RuntimeBackground": 1004, "RuntimeUI": 1005, "RuntimeUIConfiguration": 1006,
		"TutorialMemory": 2000, "TutorialData": 2001, "TutorialInstruction": 2002,
		"EngineRom": 3000,
	},
}

func memoryBlock(m mode.Mode, storage string) (int, error) {
	blocks := memoryBlocks[m]
	if blocks == nil {
		return 0, fmt.Errorf("backend: invalid mode %q", m)
	}
	block, ok := blocks[storage]
	if !ok {
		return 0, fmt.Errorf("backend: storage %q is not available in %s mode", storage, m)
	}
	return block, nil
}
