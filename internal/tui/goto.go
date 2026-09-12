package tui

import (
	"strconv"
	"strings"
	"unicode"
)

// cellAddr is spreadsheet-style, origin top-left: A1 is col 0, row 0.
func cellAddr(col, row int) string {
	if col < 0 || row < 0 {
		return ""
	}
	return string(rune('A'+col)) + strconv.Itoa(row+1)
}

func visAddr(vis, n int) string {
	if n <= 0 {
		return ""
	}
	r := vis / n
	c := vis % n
	return cellAddr(c, r)
}

// keypadFocus maps Alt+1..9 to the 3-wide phone pad (top-left of the grid).
func keypadFocus(digit, n int) int {
	if digit < 1 || digit > 9 || n <= 0 {
		return 0
	}
	d := digit - 1
	r := d / 3
	c := d % 3
	if r >= n {
		r = n - 1
	}
	if c >= n {
		c = n - 1
	}
	return r*n + c
}

// parseGoto accepts "d4" / "D4" or a 1-based linear index "16".
func parseGoto(s string, n int) (vis int, ok bool) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" || n <= 0 {
		return 0, false
	}
	if unicode.IsLetter(rune(s[0])) {
		col := int(s[0] - 'A')
		row, err := strconv.Atoi(s[1:])
		if err != nil || col < 0 || col >= n || row < 1 || row > n {
			return 0, false
		}
		return (row-1)*n + col, true
	}
	num, err := strconv.Atoi(s)
	if err != nil || num < 1 || num > n*n {
		return 0, false
	}
	return num - 1, true
}
