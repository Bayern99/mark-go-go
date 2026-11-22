package nativereview

import (
	"os"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
)

func TestMain(m *testing.M) {
	if err := portrait.RegisterFonts(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
