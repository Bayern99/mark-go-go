package main

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed scripts/feynman-rpc.mjs
var feynmanRPC []byte

func prepareResearchRelay(cmd *exec.Cmd, privateDir string) error {
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "MGO_FEYNMAN_EXECUTABLE=") {
			path := filepath.Join(privateDir, "feynman-rpc.mjs")
			if err := os.WriteFile(path, feynmanRPC, 0700); err != nil {
				return err
			}
			cmd.Env = append(cmd.Env, "PI_ACP_PI_COMMAND="+path)
			break
		}
	}
	return nil
}
