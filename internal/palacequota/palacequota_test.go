package palacequota

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeTreeUsage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := HomeTreeUsage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("got %d want 5", n)
	}
}

func TestNormalizeMax(t *testing.T) {
	if NormalizeMax(0) != 0 {
		t.Fatal()
	}
	if NormalizeMax(500) != MinBytes {
		t.Fatal()
	}
}
