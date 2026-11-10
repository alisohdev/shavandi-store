package imagestore

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalStoreOpaqueKeysAndTraversalProtection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "images")
	store, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.Save(bytes.NewBufferString("image"), "png")
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 68 {
		t.Fatalf("key=%q", key)
	}
	f, err := store.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(f)
	f.Close()
	if string(data) != "image" {
		t.Fatalf("read=%q", data)
	}
	if _, err := store.Read("..\\secret.png"); err == nil {
		t.Fatal("path traversal accepted")
	}
	info, err := os.Stat(filepath.Join(root, key))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("file mode=%o", info.Mode().Perm())
	}
	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}
}
