package frontend

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source"
)

type ErrTypeParseFailed struct {
	Pos token.Position
	Msg string

	TypeSpec *ast.TypeSpec
}

func (err *ErrTypeParseFailed) Error() string {
	return fmt.Sprintf(
		"%s:%d:%d: %s could not be parsed (%s)",
		err.Pos.Filename, err.Pos.Line, err.Pos.Column,
		source.TypeSpecString(err.TypeSpec), err.Msg,
	)
}
