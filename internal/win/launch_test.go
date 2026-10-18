//go:build windows

package win

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindKakaoExecutableSkipsMissingPathsAndDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KakaoTalk with spaces.exe")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := findKakaoExecutable([]string{filepath.Join(dir, "missing.exe"), dir, path})
	if err != nil || got != path {
		t.Fatalf("findKakaoExecutable() = %q, %v; want %q", got, err, path)
	}
}

func TestFindKakaoExecutableReportsMissingInstallation(t *testing.T) {
	if _, err := findKakaoExecutable([]string{t.TempDir()}); err == nil {
		t.Fatal("expected an error for a directory instead of an executable")
	}
}

