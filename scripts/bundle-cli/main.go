package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	log.SetFlags(0)
	for _, arch := range []string{"arm64", "amd64"} {
		if err := bundle(arch); err != nil {
			log.Fatal(err)
		}
	}
}

func bundle(arch string) error {
	dir := filepath.Join("resources", "darwin-"+arch)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join(dir, "markgogo"), "./cmd/markgogo")
	cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH="+arch, "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build bundled CLI for darwin/%s: %w", arch, err)
	}
	return nil
}
