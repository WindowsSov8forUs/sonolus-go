package engine

import "testing"

func TestSprite_ByName(t *testing.T) {
	src := "package test\n" +
		"type Skin struct { Note float64; Hold float64 }\n" +
		"type Note struct { Beat float64 `sonolus:\"imported\"` }\n" +
		"func (n *Note) Initialize() {\n" +
		"	set(0, 0, sprite(\"Note\"))\n" +
		"}\n"
	_, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("CompilePlayFile: %v", err)
	}
}

func TestSprite_SonolusPrefix(t *testing.T) {
	src := "package test\n" +
		"import \"github.com/WindowsSov8forUs/sonolus-go/sonolus\"\n" +
		"type Skin struct { Note float64; Hold float64 }\n" +
		"type Note struct { Beat float64 `sonolus:\"imported\"` }\n" +
		"func (n *Note) Initialize() {\n" +
		"	set(0, 0, sonolus.Sprite(\"Hold\"))\n" +
		"}\n"
	_, _, err := CompilePlayFile(src)
	if err != nil {
		t.Fatalf("CompilePlayFile: %v", err)
	}
}

func TestSprite_UnknownName(t *testing.T) {
	src := "package test\n" +
		"type Skin struct { Note float64 }\n" +
		"type Note struct { Beat float64 `sonolus:\"imported\"` }\n" +
		"func (n *Note) Initialize() {\n" +
		"	set(0, 0, sprite(\"DoesNotExist\"))\n" +
		"}\n"
	_, _, err := CompilePlayFile(src)
	if err == nil {
		t.Fatal("expected error for unknown sprite, got nil")
	}
	t.Logf("expected: %v", err)
}
