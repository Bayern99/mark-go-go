package macos

import (
	"strconv"
	"strings"
)

func ParseVersion(raw string) (major, minor int, ok bool) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return 0, 0, false
	}
	for _, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return 0, 0, false
		}
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	if len(parts) == 3 {
		if _, err = strconv.Atoi(parts[2]); err != nil {
			return 0, 0, false
		}
	}
	return major, minor, true
}

func supportsPDFBackground(major, minor int, ok bool) bool {
	return ok && (major > 13 || major == 13 && minor >= 3)
}
