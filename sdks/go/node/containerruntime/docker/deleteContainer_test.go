package docker

import (
	"context"
	"errors"

	"github.com/docker/docker/errdefs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/docker/internal/fakes"
)

var _ = Context("deleteContainer", func() {
	It("should not error when container removal is already in progress", func() {
		/* arrange */
		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerRemoveReturns(
			errdefs.Conflict(errors.New("removal of container opctl_build_2a647646 is already in progress")),
		)

		/* act */
		actualErr := deleteContainer(
			context.Background(),
			fakeDockerClient,
			"opctl_build_2a647646",
		)

		/* assert */
		Expect(actualErr).To(BeNil())
	})

	It("should not error when container no longer exists", func() {
		/* arrange */
		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerRemoveReturns(
			errdefs.NotFound(errors.New("No such container: opctl_build_2a647646")),
		)

		/* act */
		actualErr := deleteContainer(
			context.Background(),
			fakeDockerClient,
			"opctl_build_2a647646",
		)

		/* assert */
		Expect(actualErr).To(BeNil())
	})
})
