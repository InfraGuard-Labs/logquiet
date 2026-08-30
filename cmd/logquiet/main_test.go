package main

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/azimsiddiqui/logquiet/internal/config"
	"github.com/azimsiddiqui/logquiet/internal/pipeline"
	"github.com/azimsiddiqui/logquiet/internal/render"
)

// errReader yields data then a caller-supplied error instead of io.EOF,
// simulating a genuine read failure (a disk error, a device going away, a
// reset connection) partway through a stream - as opposed to a clean end
// of input.
type errReader struct {
	data []byte
	err  error
	pos  int
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func newTestPipeline(out io.Writer) *pipeline.Pipeline {
	ropts := render.DefaultOptions()
	ropts.Plain = true
	rnd := render.New(out, ropts)
	return pipeline.New(config.Default(), rnd)
}

// TestProcessStreamReturnsReadError is the core regression test for the
// input-read-error audit: a genuine mid-stream read failure must be
// reported by processStream, not silently treated as if the stream ended
// cleanly.
func TestProcessStreamReturnsReadError(t *testing.T) {
	wantErr := errors.New("simulated disk read error")
	r := &errReader{
		data: []byte("2026-08-30 03:01:00 [INFO] line one\n2026-08-30 03:01:01 [INFO] line two\n"),
		err:  wantErr,
	}

	var buf bytes.Buffer
	pl := newTestPipeline(&buf)
	var mu sync.Mutex

	gotErr := processStream(r, pl, &mu)

	if gotErr == nil {
		t.Fatalf("expected processStream to return the underlying read error, got nil")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("processStream returned %v, want it to wrap/match %v", gotErr, wantErr)
	}
}

// TestProcessStreamCleanEOFReturnsNil ensures the fix did not turn a
// perfectly normal end-of-input into a reported error - only a genuine
// read error should be surfaced.
func TestProcessStreamCleanEOFReturnsNil(t *testing.T) {
	r := bytes.NewBufferString("2026-08-30 03:01:00 [INFO] line one\n2026-08-30 03:01:01 [INFO] line two\n")

	var buf bytes.Buffer
	pl := newTestPipeline(&buf)
	var mu sync.Mutex

	if err := processStream(r, pl, &mu); err != nil {
		t.Fatalf("expected nil error on a clean EOF, got %v", err)
	}
}

// TestProcessStreamPartialDataBeforeErrorIsStillProcessed verifies lines
// successfully read before a later read error are not lost - a partial
// failure should not discard already-good data.
func TestProcessStreamPartialDataBeforeErrorIsStillProcessed(t *testing.T) {
	wantErr := errors.New("connection reset")
	r := &errReader{
		data: []byte("2026-08-30 03:01:00 [ERROR] disk full on /var\n"),
		err:  wantErr,
	}

	var buf bytes.Buffer
	pl := newTestPipeline(&buf)
	var mu sync.Mutex

	if err := processStream(r, pl, &mu); !errors.Is(err, wantErr) {
		t.Fatalf("expected the read error to be returned, got %v", err)
	}
	pl.Finish(time.Now())
	if pl.Counters.InputLines != 1 {
		t.Fatalf("expected the one successfully-read line to have been processed before the error, got InputLines=%d", pl.Counters.InputLines)
	}
	if !bytes.Contains(buf.Bytes(), []byte("disk full on /var")) {
		t.Fatalf("expected the successfully-read line to have reached the renderer, got %q", buf.String())
	}
}
