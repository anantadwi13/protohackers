package util

func Digits(val int32) int {
	if val == 0 {
		return 1
	}
	count := 0
	for val != 0 {
		val /= 10
		count++
	}
	return count
}
