// Package appversion reports the version of the Mark Go Go build that is
// running. The GUI, the MCP servers and the markgogo CLI all answer with it.
package appversion

// Version is the application version, which mygo.json also carries. A build
// may stamp another one with -ldflags "-X .../appversion.Version=v1.2.3".
var Version = "0.6.5-rc.1"
