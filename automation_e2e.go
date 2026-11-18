//go:build e2e

package main

import (
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
)

func init() {
	documentAutosave = mygo.AutosaveOff
}
