package hamming

import "errors"

func Distance(a, b string) (int, error) {
	i := 0
	count := 0
	if len(b) != len(a) {
		return 0, errors.New("Lengths do not match")
	}
	for i < len(a) {
		if a[i] != b[i] {
			count++
		}
		i++
	}
	return count, nil
}
