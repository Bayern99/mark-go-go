//go:build darwin

package macos

import "syscall"

func Version() (major, minor int, ok bool) {
	raw, err := syscall.Sysctl("kern.osproductversion")
	if err != nil {
		return 0, 0, false
	}
	return ParseVersion(raw)
}

func SupportsPDFBackground() bool {
	major, minor, ok := Version()
	return supportsPDFBackground(major, minor, ok)
}
