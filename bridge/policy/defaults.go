package policy

import "runtime"

// Defaults is the seed written by `devagotchi config init`. Roots start empty: nothing is reachable until a
// human names a root (fail closed, like the official filesystem server).
func Defaults() Policy {
	return Policy{
		SandboxLevel:   "workspace-write",
		Capabilities:   []string{CapRead, CapWrite, CapExec, CapProc, CapProcList},
		Shell:          defaultShell(),
		History:        true,
		FeedbackURL:    "",
		HTTPAddr:       "127.0.0.1:7676",
		PromptPatterns: []string{`>>> $`, `\.\.\. $`, `> $`, `\$ $`, `# $`, `In \[\d+\]: $`, `[a-z]+> $`, `\? $`, `PS [^>]*> $`},
		Limits:         DefaultLimits(),
		ShellProfiles:  defaultShells(),
		CodeRunners: []ShellProfile{
			{Name: "python", Argv: []string{pythonBin(), "-c"}},
			{Name: "node", Argv: []string{"node", "-e"}},
			{Name: "r", Argv: []string{"Rscript", "-e"}},
		},
	}
}

// DefaultLimits are the shipped caps (also used by the hub-connected mode).
func DefaultLimits() Limits {
	return Limits{FileReadLineLimit: 1000, MaxReadBytes: 16 << 20, DefaultTimeoutMS: 10000, MaxTimeoutMS: 3600000,
		OutputBufferBytes: 8 << 20, URLTimeoutMS: 30000, URLMaxBytes: 10 << 20, NestedDirLimit: 100,
		SearchMaxResults: 50000, SessionIdleSeconds: 3600, HistoryEntries: 1000, HistoryOutputBytes: 4096}
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "sh"
}

func defaultShells() []ShellProfile {
	if runtime.GOOS == "windows" {
		return []ShellProfile{
			{Name: "powershell", Argv: []string{"powershell.exe", "-NoLogo", "-NoProfile", "-Command"}},
			{Name: "pwsh", Argv: []string{"pwsh.exe", "-NoLogo", "-NoProfile", "-Command"}},
			{Name: "cmd", Argv: []string{"cmd.exe", "/d", "/s", "/c"}},
			{Name: "powershell-pty", Argv: []string{"powershell.exe", "-NoLogo", "-NoProfile", "-Command"}, PTY: true},
		}
	}
	return []ShellProfile{
		{Name: "sh", Argv: []string{"/bin/sh", "-c"}},
		{Name: "bash", Argv: []string{"bash", "-c"}},
		{Name: "zsh", Argv: []string{"zsh", "-c"}},
		{Name: "bash-pty", Argv: []string{"bash", "-c"}, PTY: true},
	}
}

func pythonBin() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// fill puts a default into every zero limit so a partial policy.toml still works.
func (l *Limits) fill() {
	d := DefaultLimits()
	set := func(v *int, dv int) {
		if *v <= 0 {
			*v = dv
		}
	}
	set(&l.FileReadLineLimit, d.FileReadLineLimit)
	set(&l.DefaultTimeoutMS, d.DefaultTimeoutMS)
	set(&l.MaxTimeoutMS, d.MaxTimeoutMS)
	set(&l.OutputBufferBytes, d.OutputBufferBytes)
	set(&l.URLTimeoutMS, d.URLTimeoutMS)
	set(&l.NestedDirLimit, d.NestedDirLimit)
	set(&l.SearchMaxResults, d.SearchMaxResults)
	set(&l.SessionIdleSeconds, d.SessionIdleSeconds)
	set(&l.HistoryEntries, d.HistoryEntries)
	set(&l.HistoryOutputBytes, d.HistoryOutputBytes)
	if l.MaxReadBytes <= 0 {
		l.MaxReadBytes = d.MaxReadBytes
	}
	if l.URLMaxBytes <= 0 {
		l.URLMaxBytes = d.URLMaxBytes
	}
}
