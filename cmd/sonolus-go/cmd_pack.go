package main

func cmdPack(patterns []string, name, author, romPath string, stats bool) error {
	return runPack(patterns, name, author, romPath, stats)
}
