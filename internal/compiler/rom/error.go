package rom

import (
	"fmt"
	"go/token"
)

type ErrMultiRom struct {
	Pos token.Position
}

func (err *ErrMultiRom) Error() string {
	return fmt.Sprintf(
		"%s:%d:%d: could not have multiple rom in single engine",
		err.Pos.Filename, err.Pos.Line, err.Pos.Column,
	)
}

type ErrRomFile struct {
	Pos token.Position
	Msg string
}

func (err *ErrRomFile) Error() string {
	return fmt.Sprintf(
		"%s:%d:%d: could not embed file in rom (%s)",
		err.Pos.Filename, err.Pos.Line, err.Pos.Column, err.Msg,
	)
}
