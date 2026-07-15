package volumes

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("Interpret", func() {
	Context("volume name is a literal", func() {
		It("should return expected result", func() {
			/* act */
			actual, err := Interpret(
				map[string]*model.Value{},
				map[string]string{
					"/var/lib/postgresql/data": "my-volume_1.0",
				},
			)

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(map[string]string{
				"/var/lib/postgresql/data": "my-volume_1.0",
			}))
		})
	})

	Context("volume name is a scope ref", func() {
		It("should return expected result", func() {
			/* arrange */
			volumeName := "boundVolumeName"

			/* act */
			actual, err := Interpret(
				map[string]*model.Value{
					"myVolume": {String: &volumeName},
				},
				map[string]string{
					"/data": "$(myVolume)",
				},
			)

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(map[string]string{
				"/data": volumeName,
			}))
		})
	})

	Context("volume name is interpolated", func() {
		It("should return expected result", func() {
			/* arrange */
			suffix := "pgdata"

			/* act */
			actual, err := Interpret(
				map[string]*model.Value{
					"suffix": {String: &suffix},
				},
				map[string]string{
					"/data": "myRepo-$(suffix)",
				},
			)

			/* assert */
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(map[string]string{
				"/data": "myRepo-pgdata",
			}))
		})
	})

	Context("str.Interpret errors", func() {
		It("should return expected error", func() {
			/* act */
			_, actualErr := Interpret(
				map[string]*model.Value{},
				map[string]string{
					"/data": "$(notInScope)",
				},
			)

			/* assert */
			Expect(actualErr).To(MatchError("unable to bind volume /data to $(notInScope): unable to interpret $(notInScope) to string: unable to interpret 'notInScope' as reference: 'notInScope' not in scope"))
		})
	})

	Context("volume name is empty", func() {
		It("should return expected error", func() {
			/* act */
			_, actualErr := Interpret(
				map[string]*model.Value{},
				map[string]string{
					"/data": "",
				},
			)

			/* assert */
			Expect(actualErr).To(MatchError(`unable to bind volume /data to : "" isn't a valid volume name; must match ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`))
		})
	})

	Context("container path is relative", func() {
		It("should return expected error", func() {
			/* act */
			_, actualErr := Interpret(
				map[string]*model.Value{},
				map[string]string{
					"data": "pgdata",
				},
			)

			/* assert */
			Expect(actualErr).To(MatchError("unable to bind volume data to pgdata: container path must be absolute"))
		})
	})

	Context("volume name contains invalid chars", func() {
		It("should return expected error", func() {
			/* act */
			_, actualErr := Interpret(
				map[string]*model.Value{},
				map[string]string{
					"/data": "not/a/volume",
				},
			)

			/* assert */
			Expect(actualErr).To(MatchError(`unable to bind volume /data to not/a/volume: "not/a/volume" isn't a valid volume name; must match ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`))
		})
	})
})
