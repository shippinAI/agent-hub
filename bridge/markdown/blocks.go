// Package markdown parses the block subset write_pdf and DOCX creation render: headings, paragraphs,
// bullet/numbered lists, fenced code, tables, images, rules, and page breaks. Inline marks are **bold**,
// *italic* and `code`. It is deliberately small; anything else renders as plain text.
package markdown

import (
	"regexp"
	"strings"
)

// Kind of block.
const (
	Heading   = "heading"
	Paragraph = "paragraph"
	Bullet    = "bullet"
	Numbered  = "numbered"
	Code      = "code"
	Table     = "table"
	Image     = "image"
	Rule      = "rule"
	PageBreak = "pagebreak"
)

// Block is one parsed block.
type Block struct {
	Kind  string
	Level int        // heading level
	Text  string     // paragraph/heading/list item/code text; image path
	Rows  [][]string // table cells (first row = header)
}

var (
	reHeading = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reBullet  = regexp.MustCompile(`^\s*[-*+]\s+(.*)$`)
	reNumber  = regexp.MustCompile(`^\s*\d+[.)]\s+(.*)$`)
	reImage   = regexp.MustCompile(`^!\[[^\]]*\]\(([^)\s]+)[^)]*\)\s*$|^<img\s[^>]*src="([^"]+)"[^>]*/?>\s*$`)
	reBreak   = regexp.MustCompile(`(?i)^<div[^>]*page-break-(before|after)\s*:\s*always[^>]*>\s*</div>\s*$`)
	reRule    = regexp.MustCompile(`^(\*\s*){3,}$|^(-\s*){3,}$|^(_\s*){3,}$`)
)

// Parse splits markdown into blocks.
func Parse(src string) []Block {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var out []Block
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, Block{Kind: Paragraph, Text: strings.Join(para, " ")})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "```"):
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, Block{Kind: Code, Text: strings.Join(code, "\n")})
		case t == "":
			flush()
		case reBreak.MatchString(t):
			flush()
			out = append(out, Block{Kind: PageBreak})
		case reHeading.MatchString(t):
			flush()
			m := reHeading.FindStringSubmatch(t)
			out = append(out, Block{Kind: Heading, Level: len(m[1]), Text: m[2]})
		case reRule.MatchString(t):
			flush()
			out = append(out, Block{Kind: Rule})
		case reImage.MatchString(t):
			flush()
			m := reImage.FindStringSubmatch(t)
			out = append(out, Block{Kind: Image, Text: m[1] + m[2]})
		case reBullet.MatchString(l):
			flush()
			out = append(out, Block{Kind: Bullet, Text: reBullet.FindStringSubmatch(l)[1]})
		case reNumber.MatchString(l):
			flush()
			out = append(out, Block{Kind: Numbered, Text: reNumber.FindStringSubmatch(l)[1]})
		case strings.HasPrefix(t, "|"):
			flush()
			var rows [][]string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				if row := cells(lines[i]); !isSep(row) {
					rows = append(rows, row)
				}
			}
			i--
			out = append(out, Block{Kind: Table, Rows: rows})
		default:
			para = append(para, t)
		}
	}
	flush()
	return out
}

func cells(l string) []string {
	l = strings.Trim(strings.TrimSpace(l), "|")
	parts := strings.Split(l, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSep(row []string) bool {
	for _, c := range row {
		if strings.Trim(c, ":- ") != "" {
			return false
		}
	}
	return true
}
