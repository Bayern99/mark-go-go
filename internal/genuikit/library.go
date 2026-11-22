package genuikit

import (
	_ "embed"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

//go:embed catalog.json
var catalog []byte

type Environment struct {
	Now      func() time.Time
	Location *time.Location
	Random   func() float64
}

func NewLibrary(env Environment) (*openuilang.Library, error) {
	library, err := openuilang.LoadLibrary(catalog)
	if err != nil {
		return nil, err
	}
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Location == nil {
		env.Location = time.Local
	}
	if env.Random == nil {
		env.Random = rand.Float64
	}
	env.register(library)
	return library, nil
}

var Library = sync.OnceValues(func() (*openuilang.Library, error) { return NewLibrary(Environment{}) })
