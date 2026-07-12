package directive

import (
	"go/ast"
	"go/token"
	"strings"
)

type DirectivePrefix string

const (
	PrefixGo      DirectivePrefix = "go:"
	PrefixSonolus DirectivePrefix = "sonolus:"
)

type DirectiveCmd string

const (
	CmdResource  DirectiveCmd = "resource"
	CmdCallback  DirectiveCmd = "callback"
	CmdArchetype DirectiveCmd = "archetype"
	CmdRom       DirectiveCmd = "rom"
	CmdEmbed     DirectiveCmd = "embed"
	CmdBucket    DirectiveCmd = "bucket"
)

// 编译器指令
type Directive struct {
	Cmd  DirectiveCmd
	Args []string
	Pos  token.Pos
}

func ParseDirectives(doc *ast.CommentGroup, prefix DirectivePrefix) []*Directive {
	var dirs []*Directive
	if doc == nil {
		return dirs
	}

	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, string(prefix)) {
			parts := strings.Fields(text[len(prefix):])
			if len(parts) == 0 {
				continue
			}
			dir := &Directive{
				Cmd:  DirectiveCmd(parts[0]),
				Args: parts[1:],
				Pos:  c.Slash,
			}
			dirs = append(dirs, dir)
		}
	}
	return dirs
}
