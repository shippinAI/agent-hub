package tools

// All returns every tool, DC names first, then Devagotchi additions. Order is the tools/list order.
func All() []Tool {
	return []Tool{
		getConfig(), setConfigValue(),
		readFile(), readMultipleFiles(), writeFile(), writePDF(), createDirectory(), listDirectory(), moveFile(),
		startSearch(), getMoreSearchResults(), stopSearch(), listSearches(), getFileInfo(), editBlock(),
		startProcess(), readProcessOutput(), interactWithProcess(), forceTerminate(), listSessions(), listProcesses(), killProcess(),
		getUsageStats(), getRecentToolCalls(), giveFeedback(),
		runCommand(), runCode(), deleteFile(),
	}
}

// Find returns a tool by name.
func Find(name string) (Tool, bool) {
	for _, t := range All() {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
