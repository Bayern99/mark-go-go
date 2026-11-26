package macos

import (
	"runtime"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, tc := range []struct {
		raw          string
		major, minor int
		ok           bool
	}{
		{raw: "27.2", major: 27, minor: 2, ok: true},
		{raw: "13.3.1\n", major: 13, minor: 3, ok: true},
		{raw: "14", major: 14, ok: true},
		{raw: " 12.7 ", major: 12, minor: 7, ok: true},
		{raw: ""},
		{raw: "13."},
		{raw: ".3"},
		{raw: "13.three"},
		{raw: "13.3.1.4"},
		{raw: "-13.3"},
		{raw: "13.-3"},
		{raw: "13.3.beta"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			major, minor, ok := ParseVersion(tc.raw)
			if major != tc.major || minor != tc.minor || ok != tc.ok {
				t.Fatalf("ParseVersion(%q) = %d, %d, %v; want %d, %d, %v", tc.raw, major, minor, ok, tc.major, tc.minor, tc.ok)
			}
		})
	}
}

func TestPDFBackgroundSupportUsesNumericVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		major, minor int
		ok, want     bool
	}{
		{major: 12, minor: 7, ok: true},
		{major: 13, minor: 2, ok: true},
		{major: 13, minor: 3, ok: true, want: true},
		{major: 13, minor: 10, ok: true, want: true},
		{major: 14, minor: 0, ok: true, want: true},
		{major: 27, minor: 2, ok: true, want: true},
		{major: 27, minor: 2},
	} {
		if got := supportsPDFBackground(tc.major, tc.minor, tc.ok); got != tc.want {
			t.Errorf("supportsPDFBackground(%d, %d, %v) = %v, want %v", tc.major, tc.minor, tc.ok, got, tc.want)
		}
	}
}

func TestVersionMatchesHost(t *testing.T) {
	major, minor, ok := Version()
	if runtime.GOOS != "darwin" {
		if ok || SupportsPDFBackground() {
			t.Fatalf("non-macOS version = %d.%d, %v; background support = %v", major, minor, ok, SupportsPDFBackground())
		}
		return
	}
	if !ok || major < 1 || minor < 0 {
		t.Fatalf("Version() = %d, %d, %v", major, minor, ok)
	}
	if !SupportsPDFBackground() && supportsPDFBackground(major, minor, ok) {
		t.Fatal("host supports PDF backgrounds but SupportsPDFBackground returned false")
	}
}
