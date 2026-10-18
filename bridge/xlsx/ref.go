// Package xlsx reads, writes and range-edits .xlsx/.xlsm workbooks with only the standard library (zip +
// XML), so the binary needs no spreadsheet dependency. Values are strings, numbers and booleans; formulas are
// kept on edit, and their cached values are read.
package xlsx

import (
	"fmt"
	"strconv"
	"strings"
)

// Ref is a 1-based cell position.
type Ref struct{ Col, Row int }

// ParseRef parses "B12" (absolute "$B$12" too).
func ParseRef(s string) (Ref, error) {
	s = strings.ToUpper(strings.ReplaceAll(s, "$", ""))
	i := 0
	col := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
	}
	row, err := strconv.Atoi(s[i:])
	if i == 0 || err != nil || row < 1 {
		return Ref{}, fmt.Errorf("bad cell reference %q", s)
	}
	return Ref{Col: col, Row: row}, nil
}

// String renders "B12".
func (r Ref) String() string { return ColName(r.Col) + strconv.Itoa(r.Row) }

// ColName renders 1 → A, 28 → AB.
func ColName(c int) string {
	s := ""
	for c > 0 {
		c--
		s = string(rune('A'+c%26)) + s
		c /= 26
	}
	return s
}

// Range is an inclusive rectangle, optionally naming its sheet ("Sheet1!A1:C3").
type Range struct {
	Sheet    string
	From, To Ref
}

// ParseRange accepts "A1:C3", "Sheet1!A1:C3", "'My Sheet'!B2" (a single cell is a 1x1 range).
func ParseRange(s string) (Range, error) {
	r := Range{}
	if i := strings.LastIndex(s, "!"); i >= 0 {
		r.Sheet, s = strings.Trim(s[:i], "'"), s[i+1:]
	}
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		b = a
	}
	var err error
	if r.From, err = ParseRef(a); err != nil {
		return r, err
	}
	if r.To, err = ParseRef(b); err != nil {
		return r, err
	}
	if r.To.Col < r.From.Col || r.To.Row < r.From.Row {
		r.From, r.To = r.To, r.From
	}
	return r, nil
}
