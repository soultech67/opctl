package docker

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/docker/internal/fakes"
)

var _ = Context("DeleteContainerIfExists", func() {
	It("should delete container found by opctl container ID label", func() {
		/* arrange */
		fakeDockerClient := new(FakeCommonAPIClient)

		providedCtx := context.Background()
		providedContainerID := "dummyContainerID"
		expectedContainerName := "opctl_build_dummyCon"
		fakeDockerClient.ContainerListReturns(
			[]types.Container{
				{
					Names: []string{
						"/not-opctl",
						"/" + expectedContainerName,
					},
				},
			},
			nil,
		)
		expectedContainerListOptions := container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.KeyValuePair{
					Key:   "label",
					Value: getContainerIDLabelFilter(providedContainerID),
				},
			),
		}
		expectedContainerRemoveOptions := container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		objectUnderTest.DeleteContainerIfExists(
			providedCtx,
			providedContainerID,
		)

		/* assert */
		actualCtx, actualContainerListOptions := fakeDockerClient.ContainerListArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerListOptions).To(Equal(expectedContainerListOptions))

		actualCtx, actualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(expectedContainerName))

		actualCtx,
			actualContainerName,
			actualContainerRemoveOptions := fakeDockerClient.ContainerRemoveArgsForCall(0)

		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(expectedContainerName))
		Expect(actualContainerRemoveOptions).To(Equal(expectedContainerRemoveOptions))
	})
	It("should fall back to legacy container name", func() {
		/* arrange */
		fakeDockerClient := new(FakeCommonAPIClient)

		providedCtx := context.Background()
		providedContainerID := "dummyContainerID"
		expectedContainerName := getLegacyContainerName(providedContainerID)
		expectedContainerRemoveOptions := container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		objectUnderTest.DeleteContainerIfExists(
			providedCtx,
			providedContainerID,
		)

		/* assert */
		actualCtx, actualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(expectedContainerName))

		actualCtx,
			actualContainerName,
			actualContainerRemoveOptions := fakeDockerClient.ContainerRemoveArgsForCall(0)

		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(expectedContainerName))
		Expect(actualContainerRemoveOptions).To(Equal(expectedContainerRemoveOptions))
	})
	It("should fall back to listed docker container ID when no opctl name is listed", func() {
		/* arrange */
		fakeDockerClient := new(FakeCommonAPIClient)

		providedCtx := context.Background()
		providedContainerID := "dummyContainerID"
		expectedDockerContainerID := "dockerContainerID"
		fakeDockerClient.ContainerListReturns(
			[]types.Container{
				{
					ID: expectedDockerContainerID,
					Names: []string{
						"/not-opctl",
					},
				},
			},
			nil,
		)

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		objectUnderTest.DeleteContainerIfExists(
			providedCtx,
			providedContainerID,
		)

		/* assert */
		_, actualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		Expect(actualContainerName).To(Equal(expectedDockerContainerID))

		_, actualContainerName, _ = fakeDockerClient.ContainerRemoveArgsForCall(0)
		Expect(actualContainerName).To(Equal(expectedDockerContainerID))
	})
	Context("dockerClient.ContainerRemove errors", func() {
		It("should return", func() {
			/* arrange */
			fakeDockerClient := new(FakeCommonAPIClient)
			fakeDockerClient.ContainerRemoveReturns(errors.New("dummyError"))

			objectUnderTest := _containerRuntime{
				dockerClient: fakeDockerClient,
			}

			/* act */
			actualError := objectUnderTest.DeleteContainerIfExists(
				context.Background(),
				"containerID",
			)

			/* assert */
			Expect(actualError).To(MatchError("unable to delete container: dummyError"))
		})
	})
	Context("dockerClient.ContainerRemove doesn't error", func() {
		It("shouldn't error", func() {
			/* arrange */
			objectUnderTest := _containerRuntime{
				dockerClient: new(FakeCommonAPIClient),
			}

			/* act */
			actualError := objectUnderTest.DeleteContainerIfExists(
				context.Background(),
				"containerID",
			)

			/* assert */
			Expect(actualError).To(BeNil())
		})
	})
})
