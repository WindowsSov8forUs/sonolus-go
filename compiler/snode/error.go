package snode

import "fmt"

type ErrUnknownSNodeType struct {
	SNode SNode
}

func (e ErrUnknownSNodeType) Error() string {
	return fmt.Sprintf("unknown SNode type: %T", e.SNode)
}
