package unsudo

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("ensureOwnership", func() {
	It("should recursively chown paths to the sudo user and group", func() {
		/* arrange */
		testDirPath, err := os.MkdirTemp("", "")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(testDirPath)

		nestedDirPath := filepath.Join(testDirPath, "nested")
		if err := os.Mkdir(nestedDirPath, 0700); err != nil {
			panic(err)
		}

		nestedFilePath := filepath.Join(nestedDirPath, "file")
		if err := os.WriteFile(nestedFilePath, []byte("data"), 0600); err != nil {
			panic(err)
		}

		uid := os.Geteuid()
		gid := os.Getegid()
		originalSudoUID := os.Getenv("SUDO_UID")
		originalSudoGID := os.Getenv("SUDO_GID")
		defer os.Setenv("SUDO_UID", originalSudoUID)
		defer os.Setenv("SUDO_GID", originalSudoGID)
		os.Setenv("SUDO_UID", strconv.Itoa(uid))
		os.Setenv("SUDO_GID", strconv.Itoa(gid))

		/* act */
		err = EnsureOwnership(testDirPath)

		/* assert */
		Expect(err).To(BeNil())

		for _, fsPath := range []string{testDirPath, nestedDirPath, nestedFilePath} {
			info, err := os.Lstat(fsPath)
			Expect(err).To(BeNil())

			stat := info.Sys().(*syscall.Stat_t)
			Expect(int(stat.Uid)).To(Equal(uid))
			Expect(int(stat.Gid)).To(Equal(gid))
		}
	})
})
