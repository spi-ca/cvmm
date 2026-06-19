package hvm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestNodeStatusCheckerStatusMismatch(t *testing.T) {
	errorCh := make(chan error, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"state":"Paused"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	NodeStatusChecker(context.Background(), client, model.NodeStatusRunning, errorCh)

	err, ok := <-errorCh
	if !ok || err == nil {
		t.Fatal("NodeStatusChecker() did not report status mismatch")
	}
}

func TestNodeStatusCheckerHTTPError(t *testing.T) {
	errorCh := make(chan error, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
			Header:     make(http.Header),
		}, nil
	})}

	NodeStatusChecker(context.Background(), client, model.NodeStatusRunning, errorCh)

	err, ok := <-errorCh
	if !ok || err == nil {
		t.Fatal("NodeStatusChecker() did not report http error")
	}
}

func TestNodeStatusCheckerReturnsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls atomic.Int32
	errorCh := make(chan error, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, context.Canceled
	})}

	NodeStatusChecker(ctx, client, model.NodeStatusRunning, errorCh)

	if calls.Load() != 0 {
		t.Fatalf("NodeStatusChecker() issued %d requests after cancellation, want 0", calls.Load())
	}
	if _, ok := <-errorCh; ok {
		t.Fatal("error channel remained open, want closed")
	}
}

func TestNodeStatusCheckerSuccessUntilCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	errorCh := make(chan error, 1)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"state":"Running"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	done := make(chan struct{})
	go func() {
		NodeStatusChecker(ctx, client, model.NodeStatusRunning, errorCh)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("NodeStatusChecker() did not stop after cancellation")
	}
	if calls.Load() == 0 {
		t.Fatal("NodeStatusChecker() made no successful requests")
	}
	if _, ok := <-errorCh; ok {
		t.Fatal("error channel remained open, want closed")
	}
}
