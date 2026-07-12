package main

func cmdHost(patterns []string, name, addr, author, romPath string, stats bool) error {
	return runPackServe(patterns, name, addr, author, romPath, stats)
}
