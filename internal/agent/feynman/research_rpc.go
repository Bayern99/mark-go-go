package feynman

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func prepareResearchRelay(cmd *exec.Cmd, privateDir string, relayScript []byte) error {
	if len(relayScript) == 0 {
		return nil
	}
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "MGO_FEYNMAN_EXECUTABLE=") {
			path := filepath.Join(privateDir, "feynman-rpc.mjs")
			if err := os.WriteFile(path, relayScript, 0700); err != nil {
				return err
			}
			cmd.Env = append(cmd.Env, "PI_ACP_PI_COMMAND="+path)
			break
		}
	}
	return nil
}
