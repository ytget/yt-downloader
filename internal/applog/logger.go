// Package applog provides leveled, structured application logging with
// optional file output, an in-memory ring buffer for support ("Copy logs"),
// and task-id correlation helpers.
//
// Call Init once at process start. Use L() or WithTask for slog-style logging.
// Do not log full Innertube/player JSON payloads at Info or above.
package applog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultLevel is used when level is empty or unknown.
	DefaultLevel = "info"
	// DefaultRingSize is the number of recent lines kept for CopyLogs.
	DefaultRingSize = 2000
	// LogFileName is the default log file basename under the app config dir.
	LogFileName = "app.log"
	// MaxAttrValueLen truncates very large string attrs (avoids dumping player JSON).
	MaxAttrValueLen = 512
)

// Config configures the global logger.
type Config struct {
	Level       string // debug|info|warn|error
	FileEnabled bool
	// Dir is the directory for app.log when FileEnabled. Created if missing.
	Dir string
	// RingSize overrides DefaultRingSize when > 0.
	RingSize int
	// NoStderr disables mirroring to os.Stderr (useful in tests).
	NoStderr bool
}

var (
	mu       sync.RWMutex
	logger   *slog.Logger = slog.Default()
	ring     *ringBuffer
	logPath  string
	file     *os.File
	levelVar slog.LevelVar
	inited   bool
)

// Init configures the process-wide logger. Safe to call once; later calls
// reconfigure level/file and replace handlers.
func Init(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	level := ParseLevel(cfg.Level)
	levelVar.Set(level)

	ringSize := cfg.RingSize
	if ringSize <= 0 {
		ringSize = DefaultRingSize
	}
	if ring == nil || ring.cap != ringSize {
		ring = newRingBuffer(ringSize)
	}

	// Close previous file if re-init.
	if file != nil {
		_ = file.Close()
		file = nil
	}
	logPath = ""

	var writers []io.Writer
	if !cfg.NoStderr {
		writers = append(writers, os.Stderr)
	}

	if cfg.FileEnabled {
		dir := strings.TrimSpace(cfg.Dir)
		if dir == "" {
			var err error
			dir, err = defaultLogDir()
			if err != nil {
				return err
			}
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("log dir: %w", err)
		}
		path := filepath.Join(dir, LogFileName)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		file = f
		logPath = path
		writers = append(writers, f)
	}

	var out io.Writer
	switch len(writers) {
	case 0:
		out = io.Discard
	case 1:
		out = writers[0]
	default:
		out = io.MultiWriter(writers...)
	}
	handler := &teeHandler{
		out:  slog.NewTextHandler(out, &slog.HandlerOptions{Level: &levelVar}),
		ring: ring,
	}
	logger = slog.New(handler)
	inited = true
	return nil
}

// Close flushes/closes the optional log file.
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	return err
}

// L returns the global structured logger.
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

// SetLevel updates the minimum log level at runtime.
func SetLevel(level string) {
	levelVar.Set(ParseLevel(level))
}

// Level returns the current minimum level name.
func Level() string {
	switch levelVar.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel maps a string to slog.Level.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NormalizeLevel returns a canonical level string.
func NormalizeLevel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return "debug"
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return DefaultLevel
	}
}

// WithTask returns a logger with task_id attribute for correlation.
func WithTask(taskID string) *slog.Logger {
	return L().With("task_id", taskID)
}

// Info is a convenience wrapper.
func Info(msg string, args ...any) { L().Info(msg, args...) }

// Debug is a convenience wrapper.
func Debug(msg string, args ...any) { L().Debug(msg, args...) }

// Warn is a convenience wrapper.
func Warn(msg string, args ...any) { L().Warn(msg, args...) }

// Error is a convenience wrapper.
func Error(msg string, args ...any) { L().Error(msg, args...) }

// LogFilePath returns the active log file path, or empty if file logging is off.
func LogFilePath() string {
	mu.RLock()
	defer mu.RUnlock()
	return logPath
}

// RecentText returns recent log lines from the ring buffer (newest last).
func RecentText() string {
	mu.RLock()
	r := ring
	mu.RUnlock()
	if r == nil {
		return ""
	}
	return r.Text()
}

// defaultLogDir prefers UserConfigDir/yt-downloader/logs.
func defaultLogDir() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "yt-downloader", "logs"), nil
}

// DefaultLogDir returns the standard log directory path (may not exist yet).
func DefaultLogDir() (string, error) {
	return defaultLogDir()
}

// --- ring buffer ---

type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	cap   int
	next  int
	full  bool
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, capacity),
		cap:   capacity,
	}
}

func (r *ringBuffer) Add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines[r.next] = line
	r.next = (r.next + 1) % r.cap
	if r.next == 0 {
		r.full = true
	}
}

func (r *ringBuffer) Text() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	if !r.full {
		for i := 0; i < r.next; i++ {
			b.WriteString(r.lines[i])
			if !strings.HasSuffix(r.lines[i], "\n") {
				b.WriteByte('\n')
			}
		}
		return b.String()
	}
	for i := 0; i < r.cap; i++ {
		idx := (r.next + i) % r.cap
		b.WriteString(r.lines[idx])
		if !strings.HasSuffix(r.lines[idx], "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// --- handler that tees to slog handler + ring ---

type teeHandler struct {
	out   slog.Handler
	ring  *ringBuffer
	attrs []slog.Attr
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.out.Enabled(ctx, level)
}

func (h *teeHandler) Handle(ctx context.Context, rec slog.Record) error {
	// Truncate oversized string attrs before writing (defense against JSON dumps).
	rec = sanitizeRecord(rec)
	if h.ring != nil {
		h.ring.Add(formatRecordLine(rec, h.attrs))
	}
	return h.out.Handle(ctx, rec)
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	next = append(next, h.attrs...)
	for _, a := range attrs {
		next = append(next, sanitizeAttr(a))
	}
	return &teeHandler{out: h.out.WithAttrs(attrs), ring: h.ring, attrs: next}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	// Groups are uncommon in this app; pass through without ring attr expansion.
	return &teeHandler{out: h.out.WithGroup(name), ring: h.ring, attrs: h.attrs}
}

func sanitizeRecord(rec slog.Record) slog.Record {
	out := slog.NewRecord(rec.Time, rec.Level, rec.Message, rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(sanitizeAttr(a))
		return true
	})
	return out
}

func sanitizeAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	switch a.Value.Kind() {
	case slog.KindString:
		s := a.Value.String()
		if len(s) > MaxAttrValueLen {
			a.Value = slog.StringValue(s[:MaxAttrValueLen] + "…(truncated)")
		}
		// Heuristic: drop obvious large JSON blobs (player/innertube payloads).
		if looksLikeBulkJSON(s) {
			a.Value = slog.StringValue(fmt.Sprintf("(omitted json ~%d bytes)", len(s)))
		}
	case slog.KindGroup:
		attrs := a.Value.Group()
		for i := range attrs {
			attrs[i] = sanitizeAttr(attrs[i])
		}
		a.Value = slog.GroupValue(attrs...)
	}
	return a
}

func looksLikeBulkJSON(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 2 {
		return false
	}
	if t[0] != '{' && t[0] != '[' {
		return false
	}
	// Innertube / player response fingerprints.
	if strings.Contains(t, "playerCaptions") ||
		strings.Contains(t, "captionTracks") ||
		strings.Contains(t, "playabilityStatus") ||
		strings.Contains(t, "streamingData") ||
		strings.Contains(t, "innertube") {
		return true
	}
	if t[0] == '{' && strings.Count(t, "{") > 5 && len(t) > 120 {
		return true
	}
	if t[0] == '[' && len(t) > 400 {
		return true
	}
	return false
}

func formatRecordLine(rec slog.Record, pre []slog.Attr) string {
	var b strings.Builder
	b.WriteString(rec.Time.Format(time.RFC3339))
	b.WriteByte(' ')
	b.WriteString(strings.ToUpper(rec.Level.String()))
	b.WriteByte(' ')
	b.WriteString(rec.Message)
	writeAttr := func(a slog.Attr) {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprint(a.Value.Any()))
	}
	for _, a := range pre {
		writeAttr(a)
	}
	rec.Attrs(func(a slog.Attr) bool {
		writeAttr(a)
		return true
	})
	return b.String()
}
