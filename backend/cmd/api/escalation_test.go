package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeEscalator struct {
	mu    sync.Mutex
	calls int
	errs  []error
	ticks chan struct{}
}

func (f *fakeEscalator) EscalateOverdue(context.Context) (int, error) {
	f.mu.Lock()
	var err error
	if f.calls < len(f.errs) {
		err = f.errs[f.calls]
	}
	f.calls++
	f.mu.Unlock()
	f.ticks <- struct{}{}
	return 0, err
}

func (f *fakeEscalator) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func waitTick(t *testing.T, f *fakeEscalator) {
	t.Helper()
	select {
	case <-f.ticks:
	case <-time.After(5 * time.Second):
		t.Fatal("EscalateOverdue was not called")
	}
}

func TestBR04_EscalationLoopTickInvokesEscalateOverdue(t *testing.T) {
	f := &fakeEscalator{ticks: make(chan struct{}, 10)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runEscalation(ctx, f, time.Millisecond, slog.New(slog.NewTextHandler(&lockedBuffer{}, nil)))
		close(done)
	}()
	waitTick(t, f)
	cancel()
	<-done
	if f.count() < 1 {
		t.Errorf("calls = %d, want at least 1", f.count())
	}
}

func TestBR04_EscalationLoopErrorIsLoggedAndLoopContinues(t *testing.T) {
	f := &fakeEscalator{ticks: make(chan struct{}, 10), errs: []error{errors.New("db down")}}
	logs := &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runEscalation(ctx, f, time.Millisecond, slog.New(slog.NewTextHandler(logs, nil)))
		close(done)
	}()
	waitTick(t, f) // the failing call
	waitTick(t, f) // the loop kept going
	cancel()
	<-done
	if !strings.Contains(logs.String(), "db down") {
		t.Errorf("error not logged: %q", logs.String())
	}
}

func TestBR04_EscalationLoopReturnsWhenContextCancelled(t *testing.T) {
	f := &fakeEscalator{ticks: make(chan struct{}, 10)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		runEscalation(ctx, f, time.Hour, slog.New(slog.NewTextHandler(&lockedBuffer{}, nil)))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runEscalation did not return after cancel")
	}
	if f.count() != 0 {
		t.Errorf("calls = %d, want 0 (no initial evaluation)", f.count())
	}
}
