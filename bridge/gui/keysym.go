package gui

// runeKeysym maps a rune to an X11 keysym. Latin-1 maps to itself; other printables use the Unicode
// keysym range (0x01000000 + codepoint).
func runeKeysym(r rune) uint32 {
	switch r {
	case '\n', '\r':
		return 0xff0d // Return
	case '\t':
		return 0xff09 // Tab
	}
	if r >= 0x20 && r <= 0xff {
		return uint32(r)
	}
	return 0x01000000 + uint32(r)
}

// namedKeysym maps a key name (and modifier combos like "ctrl+c") to a keysym plus modifier keysyms.
func namedKeysym(name string) (uint32, []uint32) {
	var mods []uint32
	parts := splitPlus(name)
	key := parts[len(parts)-1]
	for _, m := range parts[:len(parts)-1] {
		switch lower(m) {
		case "ctrl", "control":
			mods = append(mods, 0xffe3)
		case "shift":
			mods = append(mods, 0xffe1)
		case "alt":
			mods = append(mods, 0xffe9)
		case "super", "meta", "cmd":
			mods = append(mods, 0xffeb)
		}
	}
	if ks := special(lower(key)); ks != 0 {
		return ks, mods
	}
	if len(key) == 1 {
		return runeKeysym(rune(key[0])), mods
	}
	return 0, mods
}

func special(k string) uint32 {
	m := map[string]uint32{
		"enter": 0xff0d, "return": 0xff0d, "tab": 0xff09, "escape": 0xff1b, "esc": 0xff1b,
		"space": 0x20, "backspace": 0xff08, "delete": 0xffff, "home": 0xff50, "end": 0xff57,
		"left": 0xff51, "up": 0xff52, "right": 0xff53, "down": 0xff54, "pageup": 0xff55, "pagedown": 0xff56,
		"f1": 0xffbe, "f2": 0xffbf, "f3": 0xffc0, "f4": 0xffc1, "f5": 0xffc2, "f6": 0xffc3,
		"f7": 0xffc4, "f8": 0xffc5, "f9": 0xffc6, "f10": 0xffc7, "f11": 0xffc8, "f12": 0xffc9,
	}
	return m[k]
}

func splitPlus(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '+' && cur != "" {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	out = append(out, cur)
	return out
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}
