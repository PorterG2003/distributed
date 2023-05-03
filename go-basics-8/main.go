package spiralmatrix

// SpiralMatrix returns a spiral matrix of size n
func SpiralMatrix(n int) [][]int {
	m := make([][]int, n)
	for y := 0; y < n; y++ {
		m[y] = make([]int, n)
	}
	x, y := 0, 0
	dx, dy := 1, 0
	for count := 1; count <= n*n; count++ {
		m[y][x] = count
		xx, yy := x+dx, y+dy
		if xx < 0 || xx >= n || yy < 0 || yy >= n || m[yy][xx] != 0 {
			dx, dy = -dy, dx
		}
		x, y = x+dx, y+dy
	}
	return m
}
