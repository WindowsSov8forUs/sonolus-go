package sonolus

type VarArray[T any] struct{}

func (a VarArray[T]) Len() int                  { return 0 }
func (a VarArray[T]) Capacity() int             { return 0 }
func (a VarArray[T]) IsFull() bool              { return false }
func (a VarArray[T]) Get(index int) T           { var zero T; return zero }
func (a VarArray[T]) Set(index int, value T)    {}
func (a VarArray[T]) Append(value T)            {}
func (a VarArray[T]) Pop() T                    { var zero T; return zero }
func (a VarArray[T]) Insert(index int, value T) {}
func (a VarArray[T]) Clear()                    {}
func (a VarArray[T]) Contains(value T) bool     { return false }

type ArrayMap[K comparable, V any] struct{}

func (m ArrayMap[K, V]) Len() int            { return 0 }
func (m ArrayMap[K, V]) Capacity() int       { return 0 }
func (m ArrayMap[K, V]) Clear()              {}
func (m ArrayMap[K, V]) Get(key K) V         { var zero V; return zero }
func (m ArrayMap[K, V]) Set(key K, value V)  {}
func (m ArrayMap[K, V]) Delete(key K) bool   { return false }
func (m ArrayMap[K, V]) Contains(key K) bool { return false }

type ArraySet[T comparable] struct{}

func (s ArraySet[T]) Len() int              { return 0 }
func (s ArraySet[T]) Capacity() int         { return 0 }
func (s ArraySet[T]) Clear()                {}
func (s ArraySet[T]) Add(value T) bool      { return false }
func (s ArraySet[T]) Remove(value T) bool   { return false }
func (s ArraySet[T]) Contains(value T) bool { return false }
