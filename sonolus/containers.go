package sonolus

import "iter"

type VarArray[T any] []T

func NewVarArray[T any](capacity int) VarArray[T] { return nil }

func (a VarArray[T]) Len() int                        { return 0 }
func (a VarArray[T]) Capacity() int                   { return 0 }
func (a VarArray[T]) IsFull() bool                    { return false }
func (a VarArray[T]) Get(index int) T                 { var zero T; return zero }
func (a VarArray[T]) GetUnchecked(index int) T        { var zero T; return zero }
func (a VarArray[T]) Set(index int, value T)          {}
func (a VarArray[T]) SetUnchecked(index int, value T) {}
func (a VarArray[T]) Append(value T)                  {}
func (a VarArray[T]) AppendUnchecked(value T)         {}
func (a VarArray[T]) Pop() T                          { var zero T; return zero }
func (a VarArray[T]) Insert(index int, value T)       {}
func (a VarArray[T]) Clear()                          {}
func (a VarArray[T]) Contains(value T) bool           { return false }
func (a VarArray[T]) RemoveAt(index int) T            { var zero T; return zero }
func (a VarArray[T]) Remove(value T) bool             { return false }
func (a VarArray[T]) Index(value T) int               { return -1 }
func (a VarArray[T]) LastIndex(value T) int           { return -1 }
func (a VarArray[T]) Count(value T) int               { return 0 }
func (a VarArray[T]) Swap(i, j int)                   {}
func (a VarArray[T]) SwapUnchecked(i, j int)          {}
func (a VarArray[T]) Reverse()                        {}
func (a VarArray[T]) Shuffle()                        {}
func (a VarArray[T]) SortFunc(less func(T, T) bool)   {}
func (a VarArray[T]) Extend(values iter.Seq[T])       {}
func (a VarArray[T]) IndexMinFunc(less func(T, T) bool) int {
	return -1
}
func (a VarArray[T]) IndexMaxFunc(less func(T, T) bool) int {
	return -1
}
func (a VarArray[T]) MinFunc(less func(T, T) bool) (value T) { return value }
func (a VarArray[T]) MaxFunc(less func(T, T) bool) (value T) { return value }
func (a VarArray[T]) Values() iter.Seq[T]                    { return func(func(T) bool) {} }
func (a VarArray[T]) ValuesReversed() iter.Seq[T]            { return func(func(T) bool) {} }
func (a VarArray[T]) Items() iter.Seq2[int, T]               { return func(func(int, T) bool) {} }

func SortLinkedEntities[T any](head EntityRef[T], less func(*T, *T) bool, next func(*T) EntityRef[T], setNext func(*T, EntityRef[T])) EntityRef[T] {
	return head
}

func SortDoublyLinkedEntities[T any](head EntityRef[T], less func(*T, *T) bool, next func(*T) EntityRef[T], setNext func(*T, EntityRef[T]), setPrevious func(*T, EntityRef[T])) EntityRef[T] {
	return head
}

type ArrayMap[K comparable, V any] []Pair[K, V]

func NewArrayMap[K comparable, V any](capacity int) ArrayMap[K, V] { return nil }

func (m ArrayMap[K, V]) Len() int               { return 0 }
func (m ArrayMap[K, V]) Capacity() int          { return 0 }
func (m ArrayMap[K, V]) IsFull() bool           { return false }
func (m ArrayMap[K, V]) Clear()                 {}
func (m ArrayMap[K, V]) Get(key K) V            { var zero V; return zero }
func (m ArrayMap[K, V]) Set(key K, value V)     {}
func (m ArrayMap[K, V]) Delete(key K) bool      { return false }
func (m ArrayMap[K, V]) Contains(key K) bool    { return false }
func (m ArrayMap[K, V]) GetOK(key K) (V, bool)  { var zero V; return zero, false }
func (m ArrayMap[K, V]) Pop(key K) (V, bool)    { var zero V; return zero, false }
func (m ArrayMap[K, V]) Keys() iter.Seq[K]      { return func(func(K) bool) {} }
func (m ArrayMap[K, V]) Values() iter.Seq[V]    { return func(func(V) bool) {} }
func (m ArrayMap[K, V]) Items() iter.Seq2[K, V] { return func(func(K, V) bool) {} }

type ArraySet[T comparable] []T

func NewArraySet[T comparable](capacity int) ArraySet[T] { return nil }

func (s ArraySet[T]) Len() int              { return 0 }
func (s ArraySet[T]) Capacity() int         { return 0 }
func (s ArraySet[T]) IsFull() bool          { return false }
func (s ArraySet[T]) Clear()                {}
func (s ArraySet[T]) Add(value T) bool      { return false }
func (s ArraySet[T]) Remove(value T) bool   { return false }
func (s ArraySet[T]) Contains(value T) bool { return false }
func (s ArraySet[T]) Values() iter.Seq[T]   { return func(func(T) bool) {} }
