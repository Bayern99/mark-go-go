package pdf

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/egoist/mygo"
)

const resourcesReady = `return Promise.all([
	document.fonts ? document.fonts.ready : Promise.resolve(),
	...[...document.images].map(image => image.decode().catch(() => {}))
]).then(() => true)`

type printResult struct {
	data []byte
	err  error
}

type mygoPrinter struct{}

func (p mygoPrinter) Print(ctx context.Context, html []byte, opts Options) ([]byte, error) {
	data, err, finished := p.printAsync(ctx, html, opts)
	if finished != nil {
		<-finished
	}
	return data, err
}

func (mygoPrinter) printAsync(ctx context.Context, html []byte, opts Options) ([]byte, error, <-chan struct{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeOptions(opts)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return nil, err, nil
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "Mark Go Go Export", Hidden: true,
		Width: opts.Width, Height: opts.Height, UseContentSize: true,
		Page: mygo.PageOptions{BackgroundThrottling: mygo.ThrottlingOff},
	})
	var closeOnce sync.Once
	closeWin := func() { closeOnce.Do(func() { win.Close() }) }
	handedOff := false
	defer func() {
		if !handedOff {
			closeWin()
		}
	}()
	page := win.Page()
	loaded := make(chan error, 1)
	var loadOnce sync.Once
	finish := func(err error) { loadOnce.Do(func() { loaded <- err }) }
	offFinish := page.OnDidFinishLoad(func() { finish(nil) })
	defer offFinish()
	offFail := page.OnDidFailLoad(func(err *mygo.LoadError) { finish(err) })
	defer offFail()
	offGone := page.OnRenderProcessGone(func(reason string) {
		finish(errors.New("PDF render process ended: " + reason))
	})
	defer offGone()
	page.LoadHTML(string(html), "")
	select {
	case err := <-loaded:
		if err != nil {
			return nil, err, nil
		}
	case <-ctx.Done():
		handedOff = true
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			closeWin()
		}()
		return nil, ctx.Err(), finished
	}
	if _, err := page.EvalContext(ctx, resourcesReady); err != nil {
		if ctx.Err() != nil {
			handedOff = true
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				closeWin()
			}()
			return nil, ctx.Err(), finished
		}
		return nil, fmt.Errorf("PDF resources are not ready: %w", err), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err, nil
	}
	printed := make(chan printResult, 1)
	go func() {
		data, err := page.PrintToPDF(mygo.PDFOptions{
			PageSize: mapPaper(opts.Paper), Margins: &mygo.Margins{}, Background: opts.Background,
		})
		printed <- printResult{data: data, err: err}
	}()
	select {
	case result := <-printed:
		return result.data, result.err, nil
	case <-ctx.Done():
		handedOff = true
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			<-printed
			closeWin()
		}()
		return nil, ctx.Err(), finished
	}
}
