//go:build !darwin

package main

import (
	"errors"
	"os/exec"
)

func researchCommand(root, work string) (*exec.Cmd, error) {
	return nil, errors.New("Feynman research is available in the macOS app.")
}

func researchConsoleCommand(root, work string, args []string) (*exec.Cmd, error) {
	return nil, errors.New("The Feynman console is available in the macOS app.")
}

func stopResearchProcess(cmd *exec.Cmd) {}
