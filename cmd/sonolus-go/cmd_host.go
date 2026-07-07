package main

func cmdHost(srcPath, addr, author string) error {
	return runPackServe(srcPath, addr, author)
}
