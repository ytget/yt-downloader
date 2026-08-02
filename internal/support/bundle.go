// Package support builds sanitized diagnostics bundles for user support.
package support

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ytget/yt-downloader/internal/applog"
	"github.com/ytget/yt-downloader/internal/config"
	"github.com/ytget/yt-downloader/internal/model"
)

// Input gathers data for a support bundle.
type Input struct {
	AppVersion  string
	WailsHint   string // optional library hint
	Settings    config.SettingsData
	Tasks       []*model.DownloadTask
	RedactPaths bool
	ExtraNotes  string
}

// BuildText returns a plain-text support bundle suitable for clipboard/file.
func BuildText(in Input) string {
	var b strings.Builder
	b.WriteString("YT Downloader support bundle\n")
	b.WriteString("generated=" + time.Now().UTC().Format(time.RFC3339) + "\n")
	b.WriteString(fmt.Sprintf("app_version=%s\n", in.AppVersion))
	b.WriteString(fmt.Sprintf("go=%s\n", runtime.Version()))
	b.WriteString(fmt.Sprintf("os=%s/%s\n", runtime.GOOS, runtime.GOARCH))
	b.WriteString(fmt.Sprintf("num_cpu=%d\n", runtime.NumCPU()))
	if in.WailsHint != "" {
		b.WriteString("wails=" + in.WailsHint + "\n")
	}
	if path := applog.LogFilePath(); path != "" {
		b.WriteString("log_file=" + maybeRedactPath(path, in.RedactPaths) + "\n")
	}
	b.WriteString("\n== settings (sanitized) ==\n")
	b.WriteString(formatSettings(in.Settings, in.RedactPaths))
	b.WriteString("\n== recent task errors ==\n")
	errs := collectTaskErrors(in.Tasks, 30, in.RedactPaths)
	if len(errs) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, line := range errs {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n== recent logs ==\n")
	logs := strings.TrimSpace(applog.RecentText())
	if logs == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(logs)
		if !strings.HasSuffix(logs, "\n") {
			b.WriteByte('\n')
		}
	}
	if strings.TrimSpace(in.ExtraNotes) != "" {
		b.WriteString("\n== notes ==\n")
		b.WriteString(strings.TrimSpace(in.ExtraNotes))
		b.WriteByte('\n')
	}
	return b.String()
}

// WriteFile writes the bundle next to the log directory (or temp).
func WriteFile(text string) (string, error) {
	dir, err := applog.DefaultLogDir()
	if err != nil {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("support-bundle-%s.txt", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func formatSettings(s config.SettingsData, redact bool) string {
	dir := s.DownloadDirectory
	if redact {
		dir = redactPath(dir)
	}
	return fmt.Sprintf(
		"download_directory=%s\nmax_parallel=%d\nquality=%s\nmode=%s\ntranscript_lang=%s\ncollision=%s\nlog_level=%s\nlog_to_file=%v\nlanguage=%s\nauto_reveal=%v\nfilename_template=%s\n",
		dir,
		s.MaxParallel,
		s.QualityPreset,
		s.DownloadMode,
		s.TranscriptLang,
		s.CollisionPolicy,
		s.LogLevel,
		s.LogToFile,
		s.Language,
		s.AutoRevealComplete,
		s.FilenameTemplate,
	)
}

func collectTaskErrors(tasks []*model.DownloadTask, limit int, redact bool) []string {
	out := make([]string, 0, limit)
	for i := len(tasks) - 1; i >= 0 && len(out) < limit; i-- {
		t := tasks[i]
		if t == nil {
			continue
		}
		if t.LastError == "" && t.TranscriptError == "" && t.Warning == "" {
			continue
		}
		path := t.OutputPath
		if redact {
			path = redactPath(path)
		}
		line := fmt.Sprintf("id=%s status=%s title=%q err=%q transcript_err=%q warning=%q out=%s",
			t.ID, t.Status, truncate(t.Title, 80), t.LastError, t.TranscriptError, t.Warning, path)
		out = append(out, line)
	}
	return out
}

func maybeRedactPath(p string, redact bool) string {
	if !redact {
		return p
	}
	return redactPath(p)
}

func redactPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	// Keep basename only so support can still see file names.
	return filepath.Base(p)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
