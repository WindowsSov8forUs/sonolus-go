package main

func cmdServe(srcPath, addr, romPath string) error {
	return runDevServer(srcPath, addr, romPath)
}
