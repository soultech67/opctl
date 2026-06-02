package resolvercfg

import (
	"context"
	"os"
	"regexp"
)

// Delete modifications to the current system
func Delete(
	ctx context.Context,
) error {
	rc, err := os.ReadFile(etcResolvConfPath)
	if err != nil {
		// No resolv.conf means nothing of ours to remove.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	rcString := string(rc)

	m1 := regexp.MustCompile(`.*managed by opctl.*\n`)
	stripped := m1.ReplaceAllString(rcString, "")

	// Only rewrite when there is actually an opctl-managed line to strip. This
	// matters because Delete is now invoked on every daemon startup (resolver
	// reconciliation): rewriting unconditionally would change the file's mode
	// (to 0600) and mtime on every start, and clobber the target of a symlinked
	// resolv.conf (e.g. the systemd-resolved stub) for no reason.
	if stripped == rcString {
		return nil
	}

	return os.WriteFile(
		etcResolvConfPath,
		[]byte(stripped),
		0600,
	)
}
