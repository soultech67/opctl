package unsudo

import (
	"io/fs"
	"os"
	"path/filepath"
)

// EnsureOwnership recursively changes fsPath ownership to the user & group who ran sudo.
func EnsureOwnership(
	fsPath string,
) error {
	return filepath.WalkDir(
		fsPath,
		func(entryPath string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			return os.Lchown(entryPath, getSudoUID(), getSudoGID())
		},
	)
}
