package procs

import (
	"fmt"
	"os"
	"sort"
)

// Proc is a list_processes row. Fields an OS cannot report cheaply stay zero.
type Proc struct {
	PID        int     `json:"pid"`
	PPID       int     `json:"ppid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   int64   `json:"rss_bytes"`
	Command    string  `json:"command,omitempty"`
}

// ListAll returns every visible process, highest CPU first.
func ListAll() ([]Proc, error) {
	ps, err := listAll()
	if err != nil {
		return nil, err
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].CPUPercent != ps[j].CPUPercent {
			return ps[i].CPUPercent > ps[j].CPUPercent
		}
		return ps[i].PID < ps[j].PID
	})
	return ps, nil
}

// KillPID ends a process that is not one of ours. It refuses pid <= 1, this server and its parent.
func KillPID(pid int, force bool) error {
	if pid <= 1 || pid == os.Getpid() || pid == os.Getppid() {
		return fmt.Errorf("refusing to kill pid %d (init, this server, or its client)", pid)
	}
	return killPID(pid, force)
}
