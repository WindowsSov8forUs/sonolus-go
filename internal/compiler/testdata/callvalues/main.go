package main

const One = 1

func multiplyParameter(value float64) float64 { value *= 2; return value }
func countParameter(value float64) float64 {
	for i := 0; i < 3; i++ {
		value *= 2
	}
	return value
}
func conditionalParameter(value float64) float64 {
	count := 0
	for value < 8 && count < 4 {
		value *= 2
		count++
	}
	return value
}
func plainAssignment(value float64) float64 {
	count := 0
	for value < 8 && count < 4 {
		value = value * 2
		count++
	}
	return value
}
func incrementParameter(value int) int {
	count := 0
	for value < 4 && count < 5 {
		value++
		count++
	}
	return value
}
func branchParameter(value float64, condition bool) float64 {
	if condition {
		value *= 2
	}
	return value
}
func localControl(seed float64) float64 {
	value := seed
	return conditionalParameter(value)
}
func genericParameter[T ~int | ~float64](value T) T {
	for i := 0; i < 3; i++ {
		value *= 2
	}
	return value
}
func closureParameter() float64 {
	return func(value float64) float64 {
		for i := 0; i < 3; i++ {
			value *= 2
		}
		return value
	}(1)
}
