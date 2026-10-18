package xlsx

import (
	"path/filepath"
	"testing"
)

func TestWriteReadEdit(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b.xlsx")
	err := Write(p, []NamedGrid{{Name: "Data", Rows: [][]any{{"Name", "Age"}, {"Alice", 30.0}, {"Bob", true}}}, {Name: "Two", Rows: [][]any{{"=1+1"}}}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(p)
	if err != nil || len(b.Sheets) != 2 {
		t.Fatal(err, b)
	}
	s, _ := b.Find("0")
	g, _ := b.Grid(s)
	b.Close()
	if len(g) != 3 || g[1][0] != "Alice" || g[1][1] != 30.0 || g[2][1] != true {
		t.Fatalf("%v", g)
	}
	r, _ := ParseRange("Data!B2:C3")
	if err := SetRange(p, r, [][]any{{31.0, "x"}, {"y", nil}}); err != nil {
		t.Fatal(err)
	}
	b, _ = Open(p)
	g, _ = b.Grid(b.Sheets[0])
	b.Close()
	if g[1][1] != 31.0 || g[1][2] != "x" || g[2][1] != "y" || g[0][0] != "Name" {
		t.Fatalf("%v", g)
	}
	if ColName(28) != "AB" {
		t.Fatal(ColName(28))
	}
}
