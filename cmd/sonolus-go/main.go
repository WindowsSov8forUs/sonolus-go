package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/build"
	"github.com/WindowsSov8forUs/sonolus-go/compiler/engine"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: sonolus-go build <engine.go> [-o <out-dir>]\n")
		flag.PrintDefaults()
	}
	outDir := flag.String("o", "dist", "output directory")
	flag.Parse()

	if flag.NArg() < 1 || flag.Arg(0) != "build" {
		flag.Usage()
		os.Exit(1)
	}

	srcPath := flag.Arg(1)
	src, err := os.ReadFile(srcPath)
	if err != nil {
		fatalf("reading %s: %v", srcPath, err)
	}

	data, err := engine.CompilePlayFile(string(src))
	if err != nil {
		fatalf("compiling %s: %v", srcPath, err)
	}

	pkg, err := build.PackagePlay(&resource.EngineConfiguration{}, data)
	if err != nil {
		fatalf("packaging: %v", err)
	}

	dir := filepath.Join(*outDir, filepath.Base(srcPath[:len(srcPath)-len(filepath.Ext(srcPath))]))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatalf("creating %s: %v", dir, err)
	}
	if err := pkg.Write(dir); err != nil {
		fatalf("writing: %v", err)
	}
	fmt.Printf("wrote engine to %s/\n", dir)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
