package xlsx

import "strconv"

type xc struct {
	R  string `xml:"r,attr"`
	T  string `xml:"t,attr"`
	V  string `xml:"v"`
	IS string `xml:"is>t"`
}

// Grid reads a sheet as a dense 2D array (row 1 first). Values are string, float64 or bool.
func (b *Book) Grid(s Sheet) ([][]any, error) {
	var ws struct {
		Rows []struct {
			R     int  `xml:"r,attr"`
			Cells []xc `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := b.decode(s.Path, &ws); err != nil {
		return nil, err
	}
	var grid [][]any
	for ri, row := range ws.Rows {
		rn := row.R
		if rn == 0 {
			rn = ri + 1
		}
		for ci, c := range row.Cells {
			ref := Ref{Col: ci + 1, Row: rn}
			if c.R != "" {
				ref, _ = ParseRef(c.R)
			}
			for len(grid) < ref.Row {
				grid = append(grid, nil)
			}
			for len(grid[ref.Row-1]) < ref.Col {
				grid[ref.Row-1] = append(grid[ref.Row-1], nil)
			}
			grid[ref.Row-1][ref.Col-1] = b.value(c)
		}
	}
	return grid, nil
}

func (b *Book) value(c xc) any {
	switch c.T {
	case "s":
		if i, err := strconv.Atoi(c.V); err == nil && i < len(b.strings) {
			return b.strings[i]
		}
	case "inlineStr":
		return c.IS
	case "b":
		return c.V == "1"
	case "str", "e":
		return c.V
	}
	if f, err := strconv.ParseFloat(c.V, 64); err == nil {
		return f
	}
	if c.V == "" {
		return nil
	}
	return c.V
}
