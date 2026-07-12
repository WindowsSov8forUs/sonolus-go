package helper

func Pair[T ~float64](value T) (left, right T) {
	left = value
	right = value + 1
	return
}
