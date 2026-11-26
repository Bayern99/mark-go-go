package pdf

import (
	"context"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type Paper string

const (
	PaperA4     Paper = "a4"
	PaperLetter Paper = "letter"
)

type Options struct {
	Paper      Paper
	Background bool
	Width      int
	Height     int
	Timeout    time.Duration
}

type Printer interface {
	Print(ctx context.Context, html []byte, opts Options) ([]byte, error)
}

type asyncPrinter interface {
	printAsync(ctx context.Context, html []byte, opts Options) ([]byte, error, <-chan struct{})
}

type serializedPrinter struct {
	once  sync.Once
	sem   chan struct{}
	inner Printer
}

var Default Printer = &serializedPrinter{inner: mygoPrinter{}}

func Print(ctx context.Context, html []byte, opts Options) ([]byte, error) {
	return Default.Print(ctx, html, opts)
}

func (p *serializedPrinter) slot() chan struct{} {
	p.once.Do(func() { p.sem = make(chan struct{}, 1) })
	return p.sem
}

func (p *serializedPrinter) Print(ctx context.Context, html []byte, opts Options) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeOptions(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sem := p.slot()
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-sem
		return nil, err
	}
	if async, ok := p.inner.(asyncPrinter); ok {
		data, err, finished := async.printAsync(ctx, html, opts)
		if finished == nil {
			<-sem
		} else {
			go func() {
				<-finished
				<-sem
			}()
		}
		return data, err
	}
	defer func() { <-sem }()
	return p.inner.Print(ctx, html, opts)
}

func normalizeOptions(opts Options) Options {
	if opts.Paper != PaperLetter {
		opts.Paper = PaperA4
	}
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 1600
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Second
	}
	return opts
}

func mapPaper(paper Paper) mygo.PageSize {
	if paper == PaperLetter {
		return mygo.PageLetter
	}
	return mygo.PageA4
}
