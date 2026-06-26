package engine

import "testing"

func TestCompileWatchFile(t *testing.T) {
	src := "package p\n" +
		"type Note struct {\n\tBeat float64 `sonolus:\"imported\"`\n\tT float64 `sonolus:\"memory\"`\n}\n" +
		"func (n Note) Initialize() { n.T = n.Beat * 0.5 }\n"
	data, err := CompileWatchFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Name != "Note" {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
	if data.Archetypes[0].Initialize == nil {
		t.Fatal("missing initialize")
	}
}

func TestCompilePreviewFile(t *testing.T) {
	src := "package p\n" +
		"type Line struct {\n\tBeat float64 `sonolus:\"imported\"`\n}\n" +
		"func (l Line) Preprocess() { set(2000, 0, l.Beat) }\n"
	data, err := CompilePreviewFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Archetypes) != 1 || data.Archetypes[0].Preprocess == nil {
		t.Fatalf("archetypes=%+v", data.Archetypes)
	}
}

func TestCompileTutorialFile(t *testing.T) {
	src := "package p\n" +
		"func Preprocess() { set(2000, 0, time) }\n"
	data, err := CompileTutorialFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if data.Preprocess < 0 {
		t.Fatal("preprocess missing")
	}
}
