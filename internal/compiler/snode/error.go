package snode

import "fmt"

// ErrUnknownSNodeType is returned when a node type is not recognized.
type ErrUnknownSNodeType struct {
	SNode SNode
}

func (e ErrUnknownSNodeType) Error() string {
	return fmt.Sprintf("unknown SNode type: %T", e.SNode)
}
