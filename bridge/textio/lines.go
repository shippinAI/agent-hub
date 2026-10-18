// Package textio reads text files by line window (DC-compatible offset/length, negative offset = tail)
// without loading the whole file, and reports line counts for get_file_info.
package textio

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
)

// Window is what ReadLines returns.
type Window struct {
	Text      string `json:"-"`
	Start     int    `json:"start_line"`     // 0-based first line returned
	Returned  int    `json:"lines_returned"` // lines in Text
	Total     int    `json:"total_lines"`    // -1 when the file is too big to count cheaply
	Remaining int    `json:"remaining_lines"`
	Truncated bool   `json:"truncated_bytes,omitempty"` // maxBytes cut the window
}

// ErrBinary means the file looks binary (NUL in the first 8 KiB).
var ErrBinary = errors.New("file looks binary; read it as an image/PDF/Excel/DOCX by extension, or use a process with a suitable tool")

// countLimit bounds the full-file line count (bigger files report Total = -1).
const countLimit = 256 << 20

// ReadLines returns length lines from offset (0-based). A negative offset returns the last -offset lines.
func ReadLines(path string, offset, length int, maxBytes int64) (Window, error) {
	f, err := os.Open(path)
	if err != nil {
		return Window{}, err
	}
	defer f.Close()
	if bin, _ := IsBinary(f); bin {
		return Window{}, ErrBinary
	}
	st, _ := f.Stat()
	if offset < 0 {
		return tail(f, st.Size(), -offset, maxBytes)
	}
	_, _ = f.Seek(0, io.SeekStart)
	r := bufio.NewReaderSize(f, 64<<10)
	w := Window{Start: offset}
	var buf bytes.Buffer
	line := 0
	for {
		b, err := r.ReadSlice('\n')
		if len(b) > 0 || err == nil {
			if line >= offset && w.Returned < length {
				if int64(buf.Len()+len(b)) > maxBytes {
					w.Truncated = true
				} else {
					buf.Write(b)
					if err != bufio.ErrBufferFull {
						w.Returned++
					}
				}
			}
			if err != bufio.ErrBufferFull {
				line++
			}
		}
		if err == io.EOF || err != nil && err != bufio.ErrBufferFull {
			break
		}
		if w.Returned >= length && (st.Size() > countLimit || w.Truncated) {
			line = -1
			break
		}
	}
	w.Text, w.Total = buf.String(), line
	if line >= 0 {
		w.Remaining = max(0, line-offset-w.Returned)
	}
	return w, nil
}

func tail(f *os.File, size int64, n int, maxBytes int64) (Window, error) {
	const chunk = 64 << 10
	var data []byte
	pos := size
	for pos > 0 && bytes.Count(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) < n && int64(len(data)) < maxBytes {
		step := min(int64(chunk), pos)
		pos -= step
		b := make([]byte, step)
		if _, err := f.ReadAt(b, pos); err != nil && err != io.EOF {
			return Window{}, err
		}
		data = append(b, data...)
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if pos > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	} else if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	w := Window{Text: string(bytes.Join(lines, nil)), Returned: len(lines), Total: -1}
	if c, err := CountLines(f.Name()); err == nil {
		w.Total, w.Start = c, max(0, c-len(lines))
	}
	return w, nil
}

// IsBinary sniffs the first 8 KiB for a NUL byte.
func IsBinary(r io.ReaderAt) (bool, error) {
	b := make([]byte, 8192)
	n, err := r.ReadAt(b, 0)
	if err != nil && err != io.EOF {
		return false, err
	}
	return bytes.IndexByte(b[:n], 0) >= 0, nil
}

// CountLines counts lines (a final line without "\n" counts) for files up to countLimit.
func CountLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if st, _ := f.Stat(); st.Size() > countLimit {
		return -1, nil
	}
	buf := make([]byte, 64<<10)
	n, last := 0, byte('\n')
	for {
		c, err := f.Read(buf)
		if c > 0 {
			n += bytes.Count(buf[:c], []byte("\n"))
			last = buf[c-1]
		}
		if err != nil {
			break
		}
	}
	if last != '\n' {
		n++
	}
	return n, nil
}
