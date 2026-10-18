package markdown

import "testing"

func TestParse(t *testing.T) {
	b := Parse("# Title\n\nHello **world** and *you*.\n\n- a\n- b\n\n| h1 | h2 |\n|---|---|\n| 1 | 2 |\n\n```\ncode\n```\n<div style=\"page-break-before: always;\"></div>\n![x](/tmp/i.png)\n")
	kinds := []string{Heading, Paragraph, Bullet, Bullet, Table, Code, PageBreak, Image}
	if len(b) != len(kinds) {
		t.Fatalf("%+v", b)
	}
	for i, k := range kinds {
		if b[i].Kind != k {
			t.Fatalf("%d: %s != %s", i, b[i].Kind, k)
		}
	}
	if len(b[4].Rows) != 2 || b[7].Text != "/tmp/i.png" {
		t.Fatalf("%+v", b)
	}
	if s := Spans("a **b** `c`"); len(s) != 4 || !s[1].Bold || !s[3].Code {
		t.Fatalf("%+v", s)
	}
}
