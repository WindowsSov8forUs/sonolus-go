package snode

import (
	"strconv"
	"strings"
	"sync"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

// intSliceKeyPool reuses byte buffers for intSliceKey construction, reducing
// allocations on the hot Append path. Each buffer starts at 64 bytes (enough
// for ~16 comma-separated 32-bit ints). Storing *[]byte avoids the interface
// boxing allocation that occurs when a []byte value is passed to Pool.Put.
var intSliceKeyPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 64)
		return &b
	},
}

func intSliceKey(slice []int) string {
	if len(slice) == 0 {
		return ""
	}

	keyp := intSliceKeyPool.Get().(*[]byte)
	key := (*keyp)[:0]
	defer func() { intSliceKeyPool.Put(keyp) }()
	for i, v := range slice {
		if i > 0 {
			key = append(key, ',')
		}
		key = strconv.AppendInt(key, int64(v), 10)
	}
	return string(key)
}

// Appender builds a flat EngineDataNode list from an SNode tree, deduplicating nodes.
// It is safe for concurrent use: Append is protected by a mutex so multiple
// goroutines may call Append on the same Appender.
type Appender struct {
	mu    sync.Mutex
	nodes *[]resource.EngineDataNode

	cache map[string]int
}

// NewAppender creates a new Appender that appends nodes to the given slice.
func NewAppender(nodes *[]resource.EngineDataNode) *Appender {
	return &Appender{
		nodes: nodes,
		cache: make(map[string]int),
	}
}

// Append adds an SNode to the node list and returns its deduplicated index.
// If an equivalent node already exists, Append returns the existing index
// instead of appending a duplicate.
//
// Append is safe for concurrent use: the top-level call acquires a mutex that
// protects the shared cache and node slice. Recursive calls (from Func.Args)
// use the unexported appendLocked method which runs under the same lock.
//
// The returned error is always nil for valid Value and Func nodes. The error
// path is reserved for defensive handling of unknown SNode implementations
// (the default case in the type switch), which should never occur in practice.
func (a *Appender) Append(snode SNode) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendLocked(snode)
}

// appendLocked is the internal, non-locking implementation of Append. It is
// called from Append (under the mutex) and recursively from itself when
// processing Func argument subtrees.
func (a *Appender) appendLocked(snode SNode) (int, error) {
	switch v := snode.(type) {
	case Value:
		key := FormatNumber(float64(v))
		node := resource.EngineDataValueNode{
			Value: float64(v),
		}
		return a.appendNode(key, node), nil
	case Func:
		args := make([]int, len(v.Args))
		for i, arg := range v.Args {
			argIndex, err := a.appendLocked(arg)
			if err != nil {
				return -1, err
			}
			args[i] = argIndex
		}

		key := a.funcKey(v.Op, args)
		node := resource.EngineDataFunctionNode{
			Func: v.Op,
			Args: args,
		}
		return a.appendNode(key, node), nil
	default:
		return -1, &ErrUnknownSNodeType{SNode: v}
	}
}

func (a *Appender) appendNode(key string, node resource.EngineDataNode) int {
	if index, ok := a.cache[key]; ok {
		return index
	}

	index := len(*a.nodes)
	*a.nodes = append(*a.nodes, node)
	a.cache[key] = index

	return index
}

func (a *Appender) funcKey(fn resource.RuntimeFunction, args []int) string {
	var b strings.Builder
	b.Grow(len(fn) + 1 + len(args)*4)
	b.WriteString(string(fn))
	b.WriteByte(':')
	b.WriteString(intSliceKey(args))
	return b.String()
}
