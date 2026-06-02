package resolvercfg

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Delete modifications to the current system
func Delete(
	ctx context.Context,
) error {
	if err := filepath.WalkDir(
		resolverDir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// The resolver dir (or an entry) may have gone away; that's fine.
				if os.IsNotExist(err) {
					return nil
				}

				return err
			}

			if d == nil {
				return nil
			}

			if strings.HasPrefix(
				d.Name(),
				resolverPrefix,
			) {
				// Removal is idempotent. A concurrent per-container
				// UnregisterName (or another cleanup pass) may delete the same
				// file between WalkDir's enumeration above and this Remove, so
				// treat "already gone" as success instead of failing the whole
				// sweep (and leaving the remaining files behind).
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
			}

			return nil
		},
	); err != nil {
		return err
	}

	return clearCaches(ctx)
}
