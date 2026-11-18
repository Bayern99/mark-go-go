//go:build e2e

package main

// The official MyGo automation plugin is linked only into explicitly tagged
// test builds. Release builds also carry mygo_noinspector, which disables the
// listener even when MYGO_AUTOMATION is present.
import _ "github.com/egoist/mygo/plugins/automation"
