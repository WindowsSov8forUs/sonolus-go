package main

func cmdServe(patterns []string, name, addr, romPath string, stats bool) error {
	return runDevServer(patterns, name, addr, romPath, stats)
}
