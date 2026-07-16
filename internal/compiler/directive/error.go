package directive

import (
	"fmt"
	"go/token"
)

type ErrMultiDirectives struct {
	Pos token.Position
}

func (err *ErrMultiDirectives) Error() string {
	return fmt.Sprintf("%s:%d:%d: could not use multiple sonolus compiler directives", err.Pos.Filename, err.Pos.Line, err.Pos.Column)
}

type ErrInvalidCmd struct {
	Pos token.Position
	Msg string

	Dir *Directive
}

func (err *ErrInvalidCmd) Error() string {
	return fmt.Sprintf(
		`%s:%d:%d: invalid directive cmd "%s" (%s)`,
		err.Pos.Filename, err.Pos.Line, err.Pos.Column, err.Dir.Cmd, err.Msg,
	)
}

type ErrInvalidArg struct {
	Pos token.Position
	Msg string

	Dir *Directive
	Arg string
}

func (err *ErrInvalidArg) Error() string {
	return fmt.Sprintf(
		`%s:%d:%d: invalid arg "%s" for directive %s (%s)`,
		err.Pos.Filename, err.Pos.Line, err.Pos.Column, err.Arg, err.Dir.Cmd, err.Msg,
	)
}
