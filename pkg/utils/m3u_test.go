package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseM3UFile(t *testing.T) {
	// Create a temporary M3U file
	tmpDir := t.TempDir()
	m3uPath := filepath.Join(tmpDir, "test.m3u")

	content := `#EXTM3U
#EXTINF:180,Artist - Song 1
/music/song1.flac
#EXTINF:200,Artist - Song 2
/music/song2.flac
# Comment line
/music/song3.flac
`

	if err := os.WriteFile(m3uPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	songs, err := ParseM3UFile(m3uPath)
	if err != nil {
		t.Fatalf("ParseM3UFile failed: %v", err)
	}

	expected := 3
	if len(songs) != expected {
		t.Errorf("expected %d songs, got %d", expected, len(songs))
	}
}

func TestParseM3UFile_NotFound(t *testing.T) {
	_, err := ParseM3UFile("/nonexistent/file.m3u")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestParseM3UFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	m3uPath := filepath.Join(tmpDir, "empty.m3u")

	if err := os.WriteFile(m3uPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	songs, err := ParseM3UFile(m3uPath)
	if err != nil {
		t.Fatalf("ParseM3UFile failed: %v", err)
	}

	if len(songs) != 0 {
		t.Errorf("expected 0 songs for empty file, got %d", len(songs))
	}
}
