package logx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "debug"
	}
}

var (
	mu       sync.RWMutex
	minLevel           = LevelWarn
	out      io.Writer = io.Discard
	secrets            = make([]string, 0)
	verbose  bool
)

// SetOutput sets the destination for logs.
func SetOutput(w io.Writer) { mu.Lock(); out = w; mu.Unlock() }

// SetMinLevel sets the minimum level to emit.
func SetMinLevel(l Level) { mu.Lock(); minLevel = l; mu.Unlock() }

// SetVerbose toggles verbose output (no truncation of large fields/messages).
func SetVerbose(v bool) { mu.Lock(); verbose = v; mu.Unlock() }

// Verbose returns whether verbose output is enabled.
func Verbose() bool { mu.RLock(); defer mu.RUnlock(); return verbose }

// RegisterSecret adds a string to be redacted in outputs.
func RegisterSecret(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	mu.Lock()
	secrets = append(secrets, s)
	mu.Unlock()
}

// RegisterSecrets adds multiple secrets for redaction.
func RegisterSecrets(list []string) {
	for _, s := range list {
		RegisterSecret(s)
	}
}

// StdlogWriter wraps writes as structured JSON lines at a fixed level.
// It applies redaction and optional truncation when verbose is disabled.
func StdlogWriter(level Level, w io.Writer) io.Writer {
	if w == nil {
		w = os.Stderr
	}
	return &stdlogWriter{level: level, w: w}
}

type stdlogWriter struct {
	level Level
	w     io.Writer
}

func (sw *stdlogWriter) Write(p []byte) (int, error) {
	lines := bytes.Split(p, []byte("\n"))
	written := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if err := emit(sw.w, sw.level, string(line), nil); err != nil {
			return written, err
		}
		written += len(line) + 1 // account for newline
	}
	return written, nil
}

// Debugf logs a debug message.
func Debugf(format string, args ...any) { _ = emit(out, LevelDebug, fmt.Sprintf(format, args...), nil) }

// Infof logs an info message.
func Infof(format string, args ...any) { _ = emit(out, LevelInfo, fmt.Sprintf(format, args...), nil) }

// Warnf logs a warning message.
func Warnf(format string, args ...any) { _ = emit(out, LevelWarn, fmt.Sprintf(format, args...), nil) }

// Errorf logs an error message.
func Errorf(format string, args ...any) { _ = emit(out, LevelError, fmt.Sprintf(format, args...), nil) }

type entry struct {
	TS     string                 `json:"ts"`
	Level  string                 `json:"level"`
	Msg    string                 `json:"msg"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

func emit(w io.Writer, lvl Level, msg string, fields map[string]interface{}) error {
	mu.RLock()
	ml := minLevel
	v := verbose
	mu.RUnlock()
	if lvl < ml {
		return nil
	}
	// redact secrets
	msg = redact(msg)
	if !v {
		msg = truncate(msg, 2*1024) // 2KB default limit for non-verbose messages
	}
	// redact string fields and truncate when not verbose
	if len(fields) > 0 {
		for k, val := range fields {
			if s, ok := val.(string); ok {
				s = redact(s)
				if !v {
					s = truncate(s, 2*1024)
				}
				fields[k] = s
			}
		}
	}
	e := entry{
		TS:     time.Now().Format(time.RFC3339Nano),
		Level:  lvl.String(),
		Msg:    msg,
		Fields: fields,
	}
	b, err := json.Marshal(e)
	if err != nil {
		// fallback to plain message
		_, err2 := io.WriteString(w, msg+"\n")
		return err2
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func redact(s string) string {
	mu.RLock()
	defer mu.RUnlock()
	if len(secrets) == 0 {
		return s
	}
	out := s
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		out = strings.ReplaceAll(out, sec, "[REDACTED]")
	}
	return out
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	// keep last 10 chars to aid context
	suffix := "… [truncated]"
	if limit > len(suffix)+10 {
		head := s[:limit-len(suffix)-10]
		tail := s[len(s)-10:]
		return head + suffix + tail
	}
	return s[:limit]
}
