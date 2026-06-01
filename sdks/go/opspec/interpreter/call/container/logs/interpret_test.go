package logs

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("Interpret", func() {
	Context("log spec is nil", func() {
		It("returns nil (no log block)", func() {
			/* act */
			actual, err := Interpret(map[string]*model.Value{}, nil, "/scratch")

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual).To(BeNil())
		})
	})

	Context("rotation overrides specified", func() {
		It("carries them through and leaves Dir empty", func() {
			/* arrange */
			enabled := false
			size := 10
			backups := 2
			age := 7
			compress := false

			/* act */
			actual, err := Interpret(
				map[string]*model.Value{},
				&model.ContainerLogSpec{
					Enabled:    &enabled,
					MaxSizeMB:  &size,
					MaxBackups: &backups,
					MaxAgeDays: &age,
					Compress:   &compress,
				},
				"/scratch",
			)

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual.Dir).To(Equal(""))
			Expect(actual.Enabled).To(Equal(&enabled))
			Expect(actual.MaxSizeMB).To(Equal(&size))
			Expect(actual.MaxBackups).To(Equal(&backups))
			Expect(actual.MaxAgeDays).To(Equal(&age))
			Expect(actual.Compress).To(Equal(&compress))
		})
	})

	Context("log.dir set (host directory via scope ref)", func() {
		It("resolves Dir to the host path", func() {
			/* arrange */
			hostDir, err := os.MkdirTemp("", "")
			Expect(err).To(BeNil())
			scratch, err := os.MkdirTemp("", "")
			Expect(err).To(BeNil())
			scope := map[string]*model.Value{"myLogs": {Dir: &hostDir}}

			/* act */
			actual, err := Interpret(
				scope,
				&model.ContainerLogSpec{Dir: "$(myLogs)"},
				scratch,
			)

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual.Dir).To(Equal(hostDir))
		})
	})
})
