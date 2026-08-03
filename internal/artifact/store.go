package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Ref struct {
	Scope     string `json:"scope"`
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Store struct{ root string }

func NewStore(root string) Store { return Store{root: root} }

func (s Store) Scope() string {
	path, err := filepath.Abs(s.root)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte("agentlab.artifact-scope.v1\x00" + filepath.Clean(path)))
	return hex.EncodeToString(sum[:])
}

func (s Store) Put(data []byte) (Ref, error) {
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	ref := Ref{Scope: s.Scope(), Algorithm: "sha256", Digest: digest, Size: int64(len(data))}
	dir := filepath.Join(s.root, "sha256")
	path := filepath.Join(dir, digest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Ref{}, err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) != string(data) {
			return Ref{}, errors.New("artifact digest collision")
		}
		return ref, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Ref{}, err
	}
	if err := atomicWrite(path, data, 0o600); err != nil {
		return Ref{}, err
	}
	return ref, nil
}

func (s Store) Read(ref Ref) ([]byte, error) {
	if !ref.Valid() || ref.Scope != s.Scope() {
		return nil, errors.New("invalid artifact reference")
	}
	data, err := os.ReadFile(filepath.Join(s.root, "sha256", ref.Digest))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != ref.Digest || int64(len(data)) != ref.Size {
		return nil, errors.New("artifact content does not match reference")
	}
	return data, nil
}

func (ref Ref) Valid() bool {
	return len(ref.Scope) == sha256.Size*2 && ref.Algorithm == "sha256" && len(ref.Digest) == sha256.Size*2 && ref.Size >= 0
}

func atomicWrite(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".artifact-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}
