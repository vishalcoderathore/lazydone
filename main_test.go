package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	original := version
	version = "v0.1.0-alpha.1"
	t.Cleanup(func() { version = original })

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := stdout.String(), "lazydone v0.1.0-alpha.1\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestVersionFlagDoesNotTouchTaskFile(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(todoFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("task file should not exist, stat error = %v", err)
	}
}

func TestUnknownFlagIsAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--nope"}, &stdout, &stderr); err == nil {
		t.Fatal("run should fail on an unknown flag")
	}
	if !strings.Contains(stderr.String(), "-version") {
		t.Errorf("stderr should show usage, got %q", stderr.String())
	}
}

func TestHelpFlagShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("run error = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(stderr.String(), "-version") {
		t.Errorf("stderr should show usage, got %q", stderr.String())
	}
}
