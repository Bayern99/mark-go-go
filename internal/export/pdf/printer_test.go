package pdf

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type printerFunc func(context.Context, []byte, Options) ([]byte, error)

func (f printerFunc) Print(ctx context.Context, html []byte, opts Options) ([]byte, error) {
	return f(ctx, html, opts)
}

func TestSerializedPrinterAppliesDefaultsAndPassesResultsThrough(t *testing.T) {
	wantErr := errors.New("print failed")
	calls := 0
	printer := &serializedPrinter{inner: printerFunc(func(_ context.Context, html []byte, opts Options) ([]byte, error) {
		calls++
		if string(html) != "<p>Hello</p>" {
			t.Fatalf("HTML = %q", html)
		}
		if opts.Paper != PaperA4 || opts.Width != 1280 || opts.Height != 1600 || opts.Timeout != 20*time.Second || !opts.Background {
			t.Fatalf("normalized options = %+v", opts)
		}
		return []byte("%PDF-fake"), wantErr
	})}
	data, err := printer.Print(context.Background(), []byte("<p>Hello</p>"), Options{Background: true})
	if calls != 1 || string(data) != "%PDF-fake" || !errors.Is(err, wantErr) {
		t.Fatalf("Print() = %q, %v, calls %d", data, err, calls)
	}
}

func TestSerializedPrinterPreservesExplicitOptions(t *testing.T) {
	printer := &serializedPrinter{inner: printerFunc(func(_ context.Context, _ []byte, opts Options) ([]byte, error) {
		if opts.Paper != PaperLetter || opts.Width != 900 || opts.Height != 1200 || opts.Timeout != 3*time.Second {
			t.Fatalf("options = %+v", opts)
		}
		return nil, nil
	})}
	_, err := printer.Print(context.Background(), nil, Options{Paper: PaperLetter, Width: 900, Height: 1200, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSerializedPrinterRunsOnePrintAtATime(t *testing.T) {
	var active, peak atomic.Int32
	printer := &serializedPrinter{inner: printerFunc(func(context.Context, []byte, Options) ([]byte, error) {
		current := active.Add(1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		time.Sleep(2 * time.Millisecond)
		active.Add(-1)
		return nil, nil
	})}
	const count = 24
	var wg sync.WaitGroup
	wg.Add(count)
	for range count {
		go func() {
			defer wg.Done()
			if _, err := printer.Print(context.Background(), []byte("html"), Options{}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := peak.Load(); got != 1 {
		t.Fatalf("maximum concurrent printers = %d, want 1", got)
	}
}

type asyncPrinterFunc func(context.Context, []byte, Options) ([]byte, error, <-chan struct{})

func (f asyncPrinterFunc) Print(ctx context.Context, html []byte, opts Options) ([]byte, error) {
	data, err, finished := f(ctx, html, opts)
	if finished != nil {
		<-finished
	}
	return data, err
}

func (f asyncPrinterFunc) printAsync(ctx context.Context, html []byte, opts Options) ([]byte, error, <-chan struct{}) {
	return f(ctx, html, opts)
}

func TestSerializedPrinterRejectsCanceledContextAndQueuedWaiters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	printer := &serializedPrinter{inner: printerFunc(func(context.Context, []byte, Options) ([]byte, error) {
		called = true
		return nil, nil
	})}
	if _, err := printer.Print(ctx, nil, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if called {
		t.Fatal("canceled request entered the inner printer")
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	queued := &serializedPrinter{inner: printerFunc(func(context.Context, []byte, Options) ([]byte, error) {
		close(entered)
		<-release
		return nil, nil
	})}
	firstDone := make(chan error, 1)
	go func() {
		_, err := queued.Print(context.Background(), []byte("first"), Options{})
		firstDone <- err
	}()
	<-entered
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer waitCancel()
	if _, err := queued.Print(waitCtx, []byte("second"), Options{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued timeout error = %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first print error = %v", err)
	}
}

func TestSerializedPrinterHoldsSlotUntilAsyncPrintFinishes(t *testing.T) {
	finished := make(chan struct{})
	var secondStarted atomic.Bool
	printer := &serializedPrinter{inner: asyncPrinterFunc(func(ctx context.Context, html []byte, _ Options) ([]byte, error, <-chan struct{}) {
		if string(html) == "first" {
			return nil, context.DeadlineExceeded, finished
		}
		secondStarted.Store(true)
		return []byte("%PDF-second"), nil, nil
	})}
	if _, err := printer.Print(context.Background(), []byte("first"), Options{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first print error = %v", err)
	}
	secondDone := make(chan []byte, 1)
	go func() {
		data, err := printer.Print(context.Background(), []byte("second"), Options{})
		if err != nil {
			t.Error(err)
		}
		secondDone <- data
	}()
	time.Sleep(15 * time.Millisecond)
	if secondStarted.Load() {
		t.Fatal("second print overlapped an unfinished async print")
	}
	close(finished)
	if got := string(<-secondDone); got != "%PDF-second" {
		t.Fatalf("second print = %q", got)
	}
}

func TestPaperMapping(t *testing.T) {
	if got := mapPaper(PaperA4); got != mygo.PageA4 {
		t.Fatalf("A4 = %+v", got)
	}
	if got := mapPaper(PaperLetter); got != mygo.PageLetter {
		t.Fatalf("Letter = %+v", got)
	}
	if got := mapPaper(Paper("unknown")); got != mygo.PageA4 {
		t.Fatalf("unknown paper = %+v", got)
	}
}

func TestResourceProbeDoesNotDependOnAnimationFrames(t *testing.T) {
	for _, want := range []string{"document.fonts.ready", "image.decode().catch(() => {})"} {
		if !strings.Contains(resourcesReady, want) {
			t.Errorf("resource probe is missing %q", want)
		}
	}
	if strings.Contains(resourcesReady, "requestAnimationFrame") {
		t.Fatal("resource probe waits for animation frames in a hidden page")
	}
}
