package imagestore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var keyPattern = regexp.MustCompile(`^[a-f0-9]{64}\.(jpg|png|webp)$`)

type Store interface {
	Save(io.Reader, string) (string, error)
	Read(string) (*os.File, error)
	Delete(string) error
}
type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	if root == "" {
		return nil, errors.New("image storage directory is required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}
func (s *Local) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid image key")
	}
	return filepath.Join(s.root, key), nil
}
func (s *Local) Save(src io.Reader, ext string) (string, error) {
	if ext != "jpg" && ext != "png" && ext != "webp" {
		return "", errors.New("unsupported image type")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	key := hex.EncodeToString(random[:]) + "." + ext
	path, _ := s.path(key)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return key, nil
}
func (s *Local) Read(key string) (*os.File, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *Local) Delete(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
