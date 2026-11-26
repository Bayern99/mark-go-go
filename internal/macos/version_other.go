//go:build !darwin

package macos

func Version() (major, minor int, ok bool) {
	return 0, 0, false
}

func SupportsPDFBackground() bool {
	return false
}
