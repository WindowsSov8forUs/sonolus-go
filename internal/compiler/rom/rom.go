package rom

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/directive"
)

type RomSpec struct {
	Pkg       *packages.Package
	ValueSpec *ast.ValueSpec

	File   string
	Values []ast.Expr
}

func ASTValueSpecToRomSpec(
	pkg *packages.Package, vs *ast.ValueSpec, dir *directive.Directive,
) (*RomSpec, error) {
	var file string

	if dir != nil {
		// 取 embed
		if len(dir.Args) > 1 {
			return nil, &ErrRomFile{Pos: pkg.Fset.Position(vs.Pos()), Msg: "have multi rom file"}
		} else if len(dir.Args) <= 0 {
			return nil, &ErrRomFile{Pos: pkg.Fset.Position(vs.Pos()), Msg: "have no rom file"}
		}
		pattern := dir.Args[0]
		for _, embedFile := range pkg.EmbedFiles {
			if strings.HasSuffix(embedFile, pattern) {
				file = embedFile
				break
			}
		}
		if file == "" {
			return nil, &ErrRomFile{Pos: pkg.Fset.Position(vs.Pos()), Msg: "can not find rom file path"}
		}
	}

	return &RomSpec{Pkg: pkg, ValueSpec: vs, File: file, Values: vs.Values}, nil
}
