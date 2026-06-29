package main

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/WindowsSov8forUs/sonolus-server-go"
)

// runPackServe compiles the engine, packs it, then serves it via a real
// Sonolus server on the given address.
func runPackServe(srcPath string, addr string) error {
	if err := runPack(srcPath, "sonolus-go"); err != nil {
		return fmt.Errorf("pack: %w", err)
	}

	engineName := engineNameFromPath(srcPath)
	packDir := filepath.Join("dist", engineName+"-pack")

	s := sonolus.New(sonolus.Options{
		Address:        addr,
		FallbackLocale: "en",
	})
	if err := s.Load(packDir); err != nil {
		return fmt.Errorf("loading pack: %w", err)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	s.Install(router)

	fmt.Printf("serving on %s\n", addr)
	return http.ListenAndServe(addr, router)
}
