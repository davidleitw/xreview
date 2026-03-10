package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidleitw/xreview/internal/config"
)

func TestCollect_FilesMode_SingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	cfg := &config.Config{}
	c := NewCollector(cfg, dir)

	files, err := c.Collect(context.Background(), []string{"main.go"}, "files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Lines != 3 {
		t.Errorf("expected 3 lines, got %d", files[0].Lines)
	}
	if files[0].Content != "package main\n\nfunc main() {}\n" {
		t.Errorf("unexpected content: %q", files[0].Content)
	}
}

func TestCollect_FilesMode_Directory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	os.MkdirAll(sub, 0o755)
	writeFile(t, sub, "a.go", "package pkg\n")
	writeFile(t, sub, "b.go", "package pkg\n")

	cfg := &config.Config{}
	c := NewCollector(cfg, dir)

	files, err := c.Collect(context.Background(), []string{"pkg"}, "files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestCollect_FilesMode_IgnorePatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "main_test.go", "package main\n")

	cfg := &config.Config{
		IgnorePatterns: []string{"*_test.go"},
	}
	c := NewCollector(cfg, dir)

	files, err := c.Collect(context.Background(), []string{"main.go", "main_test.go"}, "files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file after filtering, got %d", len(files))
	}
	if filepath.Base(files[0].Path) != "main.go" {
		t.Errorf("expected main.go, got %s", files[0].Path)
	}
}

func TestCollect_UnknownMode(t *testing.T) {
	cfg := &config.Config{}
	c := NewCollector(cfg, t.TempDir())

	_, err := c.Collect(context.Background(), []string{"foo"}, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestCollect_NoFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		IgnorePatterns: []string{"*.go"},
	}
	c := NewCollector(cfg, dir)
	writeFile(t, dir, "main.go", "package main\n")

	_, err := c.Collect(context.Background(), []string{"main.go"}, "files")
	if err == nil {
		t.Fatal("expected error when all files filtered out")
	}
}

func TestCollect_LineCount_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "notrail.txt", "line1\nline2")

	cfg := &config.Config{}
	c := NewCollector(cfg, dir)

	files, err := c.Collect(context.Background(), []string{"notrail.txt"}, "files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files[0].Lines != 2 {
		t.Errorf("expected 2 lines, got %d", files[0].Lines)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
