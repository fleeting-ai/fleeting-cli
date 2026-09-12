package tui

import "fmt"

// Column-major index: down a column, then the next column to the right.
// Full page = n columns; column page = 1 column.

func globalIndex(colOff, n, col, row int) int {
	return (colOff+col)*n + row
}

func pageOf(index, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	return index / pageSize
}

func pageLabel(colOff, n int) string {
	if n <= 0 {
		return "Page 1"
	}
	pageSize := n * n
	lo, hi := -1, -1
	for c := 0; c < n; c++ {
		for r := 0; r < n; r++ {
			p := pageOf(globalIndex(colOff, n, c, r), pageSize)
			if lo < 0 || p < lo {
				lo = p
			}
			if p > hi {
				hi = p
			}
		}
	}
	if lo == hi {
		return fmt.Sprintf("Page %d", lo+1)
	}
	return fmt.Sprintf("Page %d-%d", lo+1, hi+1)
}

func maxColOffset(agentCount, n int) int {
	if n <= 0 {
		return 0
	}
	colsUsed := (agentCount + n - 1) / n
	if colsUsed < n {
		colsUsed = n
	}
	if colsUsed < 2*n {
		colsUsed = 2 * n
	}
	return colsUsed - n
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
