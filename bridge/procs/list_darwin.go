package procs

import (
	"os/exec"
	"strconv"
	"strings"
)

// listAll uses /bin/ps, which every macOS ships (libproc would need cgo).
func listAll() ([]Proc, error) {
	b, err := exec.Command("/bin/ps", "-axo", "pid=,ppid=,pcpu=,rss=,user=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var out []Proc
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		p := Proc{User: f[4], Command: strings.Join(f[5:], " ")}
		p.PID, _ = strconv.Atoi(f[0])
		p.PPID, _ = strconv.Atoi(f[1])
		p.CPUPercent, _ = strconv.ParseFloat(f[2], 64)
		kb, _ := strconv.ParseInt(f[3], 10, 64)
		p.RSSBytes = kb * 1024
		p.Name = p.Command[strings.LastIndexByte(p.Command, '/')+1:]
		out = append(out, p)
	}
	return out, nil
}
