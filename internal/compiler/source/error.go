package source

import (
	"fmt"
	"go/token"

	sourcetracer "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/source/tracer"
	compilerTag "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/tag"
)

type ErrInvalidTagOption struct {
	Pos token.Position
	Msg string

	Tag    *compilerTag.Value
	Option string
}

func (err *ErrInvalidTagOption) Error() string {
	return fmt.Sprintf(
		`%s:%d:%d: invalid option "%s" for tag %s (%s)`,
		err.Pos.Filename, err.Pos.Line, err.Pos.Column, err.Option, err.Tag.Name, err.Msg,
	)
}

type ErrExprProcessFailed = sourcetracer.ErrExprProcessFailed
type StaticEvalError = sourcetracer.StaticEvalError

var (
	ErrNotStatic       = sourcetracer.ErrNotStatic
	ErrStaticCycle     = sourcetracer.ErrStaticCycle
	ErrStaticPanic     = sourcetracer.ErrStaticPanic
	ErrMissingTypeInfo = sourcetracer.ErrMissingTypeInfo
)
