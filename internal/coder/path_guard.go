package coder

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func securePath(root, name string, allowNew bool) (string, error) {
	if !filepath.IsAbs(root) || !safe(name) {
		return "", errors.New("coder path is invalid")
	}
	segments := strings.Split(filepath.ToSlash(name), "/")
	current := root
	for index, segment := range segments {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && allowNew {
			if index < len(segments)-1 {
				if err := os.Mkdir(current, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
					return "", err
				}
				continue
			}
			return current, nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (index < len(segments)-1 && !info.IsDir()) {
			return "", errors.New("coder path escapes workspace")
		}
	}
	return current, nil
}

func safe(name string) bool {
	clean := path.Clean(filepath.ToSlash(name))
	return name != "" && name == clean && !path.IsAbs(name) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(name, `\`)
}
