package nativeeditor

import "testing"

func TestEscapeCellPreservesRawBytes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "a|b", want: `a\|b`},
		{in: `a\|b`, want: `a\|b`},
		{in: `a\\|b`, want: `a\\\|b`},
		{in: "one\r\ntwo\rthree\nfour", want: "one<br>two<br>three<br>four"},
		{in: "中文🙂|cell", want: `中文🙂\|cell`},
		{in: "raw\xff|x\xfe\r\ny\rz\n", want: "raw\xff\\|x\xfe<br>y<br>z<br>"},
	}
	for _, tc := range cases {
		if got := escapeCell(tc.in); got != tc.want {
			t.Errorf("escapeCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
