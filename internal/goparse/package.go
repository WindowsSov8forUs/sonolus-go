// Package goparse provides pure Go source parsing, producing a clean IR
// with no Sonolus engine semantics. It is the foundation layer that the
// engine compiler consumes.
package goparse

import (
	"go/ast"
	"go/token"
)

// Package represents a single parsed Go package.
type Package struct {
	// Name is the Go package declaration name (e.g. "notes" for `package notes`).
	Name string

	// Fset holds source position information for all parsed files, enabling
	// downstream consumers to produce diagnostics with source locations.
	Fset *token.FileSet

	// Files holds the parsed files in declaration order.
	Files []*File
}

// File represents a single parsed Go source file.
type File struct {
	// Name is the file name (e.g. "tap.go").
	Name string

	// Types holds all struct type declarations found in this file.
	Types []*TypeDecl

	// Funcs holds all free function declarations (no receiver).
	Funcs []*FuncDecl

	// Methods holds all method declarations (functions with a receiver).
	Methods []*MethodDecl

	// Vars holds all variable declarations, including composite literal
	// initializers.
	Vars []*VarDecl
}

// TypeDecl represents a Go struct type declaration.
type TypeDecl struct {
	// Name is the type name (e.g. "Note").
	Name string

	// Fields is the list of struct fields. Fields declared on the same line
	// (e.g. `a, b float64`) share the same Type and Tag.
	Fields []*Field
}

// Field represents a struct field in a type declaration.
type Field struct {
	// Names holds the field names. Multiple names per entry are supported
	// (Go allows `a, b float64` on one line).
	Names []string

	// Type is a human-readable representation of the field's Go type,
	// produced by rendering the AST type expression.
	Type string

	// TypeExpr is the raw AST type expression from the Go parser, preserved
	// so downstream consumers that need AST access (e.g. for nested struct
	// types in bucket definitions) can work without re-parsing.
	TypeExpr ast.Expr

	// Tag is the raw struct tag string (including backtick quotes), or empty
	// if no tag is present. Callers parse tags as needed.
	Tag string
}

// FuncDecl represents a free function declaration (no receiver).
type FuncDecl struct {
	// Name is the function name.
	Name string

	// Body is the function body block, or nil if the function has no body
	// (external declaration or forward reference).
	Body *ast.BlockStmt

	// Params holds the function parameter list.
	Params []*Param
}

// MethodDecl represents a method declaration on a type.
type MethodDecl struct {
	// ReceiverType is the receiver type name (e.g. "Note"). For pointer
	// receivers like `*Note`, the `*` is stripped.
	ReceiverType string

	// ReceiverName is the receiver variable name (e.g. "n").
	ReceiverName string

	// MethodName is the method name (e.g. "Initialize").
	MethodName string

	// Body is the method body block, or nil if the method has no body.
	Body *ast.BlockStmt
}

// VarDecl represents a Go variable declaration.
type VarDecl struct {
	// Names holds the variable names.
	Names []string

	// Type is the explicit type name (e.g. "UI"), or empty if the type is
	// inferred from the initializer.
	Type string

	// Values holds the initializer expressions. May be nil for zero-value
	// declarations.
	Values []ast.Expr
}

// Param represents a function parameter.
type Param struct {
	// Names holds the parameter names.
	Names []string

	// Type is a human-readable representation of the parameter's Go type.
	Type string
}
