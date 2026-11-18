//go:build e2e

package main

// The official MyGo automation plugin is linked only into explicitly tagged
// test builds. Release builds also carry mygo_noinspector, which disables the
// listener even when MYGO_AUTOMATION is present.
import (
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
)

func init() {
	// Make File > Save the only way the smoke's edit can reach disk.
	documentAutosave = mygo.AutosaveOff
}
