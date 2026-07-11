package tracer

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type ErrExprProcessFailed struct {
	Pos token.Position
	Msg string

	Expr ast.Expr
}

func (err *ErrExprProcessFailed) Error() string {
	return fmt.Sprintf(
		`%s:%d:%d: failed to process type "%s" (%s)`,
		err.Pos.Filename, err.Pos.Line, err.Pos.Column, formatExpr(err.Expr), err.Msg,
	)
}

var (
	ErrNotStatic       = errors.New("expression is not statically evaluable")
	ErrStaticCycle     = errors.New("static initialization cycle")
	ErrStaticPanic     = errors.New("static evaluation would panic")
	ErrMissingTypeInfo = errors.New("package is missing syntax or type information")
)

// StaticEvalError reports the source expression and declaration being evaluated.
type StaticEvalError struct {
	Pos    token.Position
	Expr   ast.Expr
	Object types.Object
	Err    error
}

func (err *StaticEvalError) Error() string {
	subject := "static value"
	if err.Expr != nil {
		subject = fmt.Sprintf("expression %q", staticExprString(err.Expr))
	} else if err.Object != nil {
		subject = fmt.Sprintf("object %q", err.Object.Name())
	}
	if err.Pos.IsValid() {
		return fmt.Sprintf(
			"%s:%d:%d: failed to evaluate %s (%v)",
			err.Pos.Filename, err.Pos.Line, err.Pos.Column, subject, err.Err,
		)
	}
	return fmt.Sprintf("failed to evaluate %s (%v)", subject, err.Err)
}

func (err *StaticEvalError) Unwrap() error {
	return err.Err
}

func staticExprString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expr); err != nil {
		return formatExpr(expr)
	}
	return buffer.String()
}

func formatExpr(expr ast.Expr) string {
	if expr == nil {
		return "<nil>"
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expr); err != nil {
		return fmt.Sprintf("<%T>", expr)
	}
	return buffer.String()
}

func (e *staticEvaluator) errorAt(pkg *packages.Package, expr ast.Expr, object types.Object, cause error) error {
	var position token.Position
	if pkg != nil && pkg.Fset != nil {
		switch {
		case expr != nil:
			position = pkg.Fset.Position(expr.Pos())
		case object != nil:
			position = pkg.Fset.Position(object.Pos())
		}
	}
	return &StaticEvalError{Pos: position, Expr: expr, Object: object, Err: cause}
}

func (e *staticEvaluator) annotateError(pkg *packages.Package, expr ast.Expr, err error) error {
	var evalErr *StaticEvalError
	if errors.As(err, &evalErr) && evalErr.Pos.IsValid() {
		return err
	}
	return e.errorAt(pkg, expr, nil, err)
}

func (e *staticEvaluator) associateError(pkg *packages.Package, object types.Object, err error) error {
	var evalErr *StaticEvalError
	if errors.As(err, &evalErr) {
		associated := *evalErr
		if associated.Object == nil {
			associated.Object = object
		}
		return &associated
	}
	return e.errorAt(pkg, nil, object, err)
}
