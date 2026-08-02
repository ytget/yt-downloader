// Package netutil provides transient-error classification, retries, and
// user-facing network error messages.
package netutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Phase names for timeouts / logging.
const (
	PhaseResolve  = "resolve"
	PhaseMedia    = "media"
	PhaseCaptions = "captions"
)

// Default timeouts per download phase.
const (
	DefaultResolveTimeout  = 45 * time.Second
	DefaultMediaTimeout    = 0 // 0 = no extra deadline beyond cancel (long downloads)
	DefaultCaptionsTimeout = 90 * time.Second
)

// RetryConfig controls backoff retries for transient failures.
type RetryConfig struct {
	Attempts int
	Base     time.Duration
	MaxDelay time.Duration
}

// DefaultRetry is used for resolve / captions / short media probes.
func DefaultRetry() RetryConfig {
	return RetryConfig{Attempts: 3, Base: 400 * time.Millisecond, MaxDelay: 4 * time.Second}
}

// IsTransient reports whether err is worth retrying.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// DeadlineExceeded can be transient for the operation; still allow retry
		// only when not parent-canceled.
		if errors.Is(err, context.Canceled) {
			return false
		}
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return IsTransient(urlErr.Err)
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, p := range []string{
		"timeout", "temporar", "connection reset", "connection refused",
		"tls handshake", "broken pipe", "i/o timeout", "eof",
		"http 429", "http 502", "http 503", "http 504",
		"too many requests", "service unavailable", "bad gateway",
	} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// Do retries fn when it returns a transient error.
func Do(ctx context.Context, cfg RetryConfig, fn func(context.Context) error) error {
	if cfg.Attempts < 1 {
		cfg.Attempts = 1
	}
	if cfg.Base <= 0 {
		cfg.Base = 300 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 5 * time.Second
	}
	var last error
	delay := cfg.Base
	for i := 0; i < cfg.Attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = fn(ctx)
		if last == nil {
			return nil
		}
		if errors.Is(last, context.Canceled) {
			return last
		}
		if i == cfg.Attempts-1 || !IsTransient(last) {
			return last
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		delay *= 2
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}
	return last
}

// UserMessage maps network/system errors to short UI strings.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "Cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Timed out — check your connection and try again"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS lookup failed — check your internet connection"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "dns"):
		return "DNS lookup failed — check your internet connection"
	case strings.Contains(msg, "network is unreachable"), strings.Contains(msg, "offline"):
		return "Network unreachable — you appear to be offline"
	case strings.Contains(msg, "connection refused"):
		return "Connection refused — service may be unavailable"
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "tls"), strings.Contains(msg, "x509"):
		return "Secure connection failed (TLS/certificate error)"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "i/o timeout"):
		return "Network timeout — please try again"
	case strings.Contains(msg, "429"), strings.Contains(msg, "too many requests"):
		return "Rate limited by server — wait a moment and retry"
	case strings.Contains(msg, "502"), strings.Contains(msg, "503"), strings.Contains(msg, "504"):
		return "Server temporarily unavailable — try again shortly"
	}
	if IsTransient(err) {
		return "Temporary network error — please try again"
	}
	return err.Error()
}

// WithTimeout returns ctx with timeout when d > 0; otherwise returns ctx + noop cancel.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, d)
}

// FormatAttempt annotates errors after retries (debug-friendly).
func FormatAttempt(phase string, attempt, max int, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s failed (attempt %d/%d): %v", phase, attempt, max, err)
}
