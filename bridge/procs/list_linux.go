package procs

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const clkTck = 100 // USER_HZ on every Linux target we build

func listAll() ([]Proc, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	up := uptime()
	pageSize := int64(os.Getpagesize())
	users := map[string]string{}
	var out []Proc
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		s := string(stat)
		lp, rp := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		if lp < 0 || rp < lp {
			continue
		}
		f := strings.Fields(s[rp+1:])
		if len(f) < 22 {
			continue
		}
		p := Proc{PID: pid, Name: s[lp+1 : rp]}
		p.PPID, _ = strconv.Atoi(f[1])
		ut, _ := strconv.ParseFloat(f[11], 64)
		st, _ := strconv.ParseFloat(f[12], 64)
		start, _ := strconv.ParseFloat(f[19], 64)
		rss, _ := strconv.ParseInt(f[21], 10, 64)
		if age := up - start/clkTck; age > 0 {
			p.CPUPercent = float64(int((ut+st)/clkTck/age*1000)) / 10
		}
		p.RSSBytes = rss * pageSize
		if cl, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline")); err == nil {
			p.Command = strings.TrimSpace(strings.ReplaceAll(string(cl), "\x00", " "))
		}
		p.User = owner(filepath.Join("/proc", e.Name()), users)
		out = append(out, p)
	}
	return out, nil
}

func uptime() float64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return float64(time.Now().Unix())
	}
	v, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
	return v
}

func owner(dir string, cache map[string]string) string {
	fi, err := os.Stat(dir)
	if err != nil {
		return ""
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	uid := strconv.Itoa(int(st.Uid))
	if n, ok := cache[uid]; ok {
		return n
	}
	n := uid
	if u, err := user.LookupId(uid); err == nil {
		n = u.Username
	}
	cache[uid] = n
	return n
}
