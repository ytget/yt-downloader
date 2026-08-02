package netutil

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIsTransient(t *testing.T) {
	if IsTransient(nil) {
		t.Fatal("nil")
	}
	if !IsTransient(errors.New("connection reset by peer")) {
		t.Fatal("reset")
	}
	if !IsTransient(errors.New("HTTP 503 service unavailable")) {
		t.Fatal("503")
	}
	if IsTransient(errors.New("video is private")) {
		t.Fatal("private should not retry")
	}
	if IsTransient(context.Canceled) {
		t.Fatal("canceled")
	}
	if !IsTransient(&net.DNSError{IsTimeout: true, Err: "timeout", Name: "x"}) {
		t.Fatal("dns timeout")
	}
}

func TestDoRetriesTransient(t *testing.T) {
	var n int
	err := Do(context.Background(), RetryConfig{Attempts: 3, Base: time.Millisecond, MaxDelay: time.Millisecond}, func(ctx context.Context) error {
		n++
		if n < 3 {
			return errors.New("timeout waiting")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("n=%d", n)
	}
}

func TestDoNoRetryPermanent(t *testing.T) {
	var n int
	err := Do(context.Background(), RetryConfig{Attempts: 5, Base: time.Millisecond}, func(ctx context.Context) error {
		n++
		return errors.New("video is private")
	})
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("err=%v", err)
	}
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
}

func TestUserMessage(t *testing.T) {
	if !strings.Contains(UserMessage(errors.New("no such host")), "DNS") {
		t.Fatal(UserMessage(errors.New("no such host")))
	}
	if !strings.Contains(strings.ToLower(UserMessage(errors.New("x509: certificate"))), "tls") {
		t.Fatal(UserMessage(errors.New("x509: certificate")))
	}
}
