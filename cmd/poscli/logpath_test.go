package main

import (
	"os"
	"strings"
	"testing"

	"github.com/yourname/poscli/internal/closelog"
)

func TestResolveLogFile_FlagWins(t *testing.T) {
	t.Setenv(closelog.EnvOverride, "/env/path.log")
	got := resolveLogFile("/flag/path.log", "/config/path.log")
	if got != "/flag/path.log" {
		t.Errorf("flag should win, got %q", got)
	}
}

func TestResolveLogFile_EnvBeatsConfig(t *testing.T) {
	t.Setenv(closelog.EnvOverride, "/env/path.log")
	got := resolveLogFile("", "/config/path.log")
	if got != "/env/path.log" {
		t.Errorf("env should beat config, got %q", got)
	}
}

func TestResolveLogFile_ConfigBeatsDefault(t *testing.T) {
	t.Setenv(closelog.EnvOverride, "")
	got := resolveLogFile("", "/config/path.log")
	if got != "/config/path.log" {
		t.Errorf("config should beat default, got %q", got)
	}
}

func TestResolveLogFile_FallsBackToDefault(t *testing.T) {
	t.Setenv(closelog.EnvOverride, "")
	got := resolveLogFile("", "")
	want := closelog.DefaultPath()
	if got != want {
		t.Errorf("expected default %q, got %q", want, got)
	}
}

func TestResolveLogFile_DefaultPathLooksSensible(t *testing.T) {
	p := closelog.DefaultPath()
	if !strings.HasSuffix(p, "close.log") {
		t.Errorf("default path should end with close.log, got %q", p)
	}
	// HOME-based path 或 fallback "close.log"
	if home, err := os.UserHomeDir(); err == nil {
		if !strings.HasPrefix(p, home) {
			t.Errorf("expected path under HOME=%q, got %q", home, p)
		}
	}
}
