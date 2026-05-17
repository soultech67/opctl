package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/docker/internal/fakes"
)

var _ = Context("deleteOpctlContainers", func() {
	It("should delete opctl containers by docker name", func() {
		/* arrange */
		providedCtx := context.Background()
		expectedContainerName := "opctl_build_2a647646"
		expectedContainerListOptions := container.ListOptions{
			All:     true,
			Filters: getOpctlContainerFilters(),
		}

		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerListReturns(
			[]types.Container{
				{
					Names: []string{
						"/" + expectedContainerName,
						"/" + expectedContainerName + "-alias",
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
		actualErr := objectUnderTest.deleteOpctlContainers(
			providedCtx,
			getOpctlContainerFilters(),
		)

		/* assert */
		Expect(actualErr).To(BeNil())

		actualCtx, actualContainerListOptions := fakeDockerClient.ContainerListArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerListOptions).To(Equal(expectedContainerListOptions))

		_, actualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		Expect(actualContainerName).To(Equal(expectedContainerName))

		_, actualContainerName, _ = fakeDockerClient.ContainerRemoveArgsForCall(0)
		Expect(actualContainerName).To(Equal(expectedContainerName))

		Expect(fakeDockerClient.ContainerStopCallCount()).To(Equal(1))
		Expect(fakeDockerClient.ContainerRemoveCallCount()).To(Equal(1))
	})
})

var _ = Context("DeleteContainer", func() {
	It("should delete the specified container", func() {
		/* arrange */
		providedCtx := context.Background()
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"

		fakeDockerClient := new(FakeCommonAPIClient)

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		actualErr := objectUnderTest.DeleteContainer(
			providedCtx,
			providedContainerID,
		)

		/* assert */
		Expect(actualErr).To(BeNil())

		actualCtx, actualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(providedContainerID))

		_, actualContainerName, _ = fakeDockerClient.ContainerRemoveArgsForCall(0)
		Expect(actualContainerName).To(Equal(providedContainerID))
	})
})

var _ = Context("DeleteContainersByLabels", func() {
	It("should delete every opctl container matching provided labels", func() {
		/* arrange */
		providedCtx := context.Background()
		providedLabels := []string{
			"opctl.container-name=localstack",
			"opctl.image-ref=localstack/localstack-pro:latest",
		}
		expectedFirstContainerName := "opctl_localstack_2a647646"
		expectedSecondContainerName := "opctl_localstack_b2fce04c"
		expectedContainerListOptions := container.ListOptions{
			All:     true,
			Filters: getOpctlContainerLabelFilters(providedLabels),
		}

		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerListReturns(
			[]types.Container{
				{
					Names: []string{
						"/" + expectedFirstContainerName,
					},
				},
				{
					Names: []string{
						"/" + expectedSecondContainerName,
					},
				},
			},
			nil,
		)

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		actualErr := objectUnderTest.DeleteContainersByLabels(
			providedCtx,
			providedLabels,
		)

		/* assert */
		Expect(actualErr).To(BeNil())

		actualCtx, actualContainerListOptions := fakeDockerClient.ContainerListArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerListOptions).To(Equal(expectedContainerListOptions))

		_, firstActualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(0)
		_, secondActualContainerName, _ := fakeDockerClient.ContainerStopArgsForCall(1)
		Expect([]string{
			firstActualContainerName,
			secondActualContainerName,
		}).To(ConsistOf(
			expectedFirstContainerName,
			expectedSecondContainerName,
		))

		Expect(fakeDockerClient.ContainerStopCallCount()).To(Equal(2))
		Expect(fakeDockerClient.ContainerRemoveCallCount()).To(Equal(2))
	})
})

var _ = Context("ListContainersByLabels", func() {
	It("should list opctl containers matching provided labels", func() {
		/* arrange */
		providedCtx := context.Background()
		providedLabels := []string{
			"opctl.container-name=localstack",
			"opctl.image-ref=localstack/localstack-pro:latest",
		}
		expectedContainerID := "2a647646e9cc4ef4940f52ad944f5657"
		expectedContainerName := "opctl_localstack_2a647646"
		expectedImage := "localstack/localstack-pro:latest"
		expectedStartedAt := time.Date(2026, 5, 17, 17, 1, 32, 0, time.UTC)
		expectedLabels := map[string]string{
			"opctl.container-id":   expectedContainerID,
			"opctl.container-name": "localstack",
			"opctl.image-ref":      expectedImage,
			"opctl.managed":        "true",
		}
		expectedContainerListOptions := container.ListOptions{
			All:     true,
			Filters: getOpctlContainerLabelFilters(providedLabels),
		}

		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerListReturns(
			[]types.Container{
				{
					ID:      expectedContainerID,
					Image:   expectedImage,
					Labels:  expectedLabels,
					Created: expectedStartedAt.Add(-time.Minute).Unix(),
					Names: []string{
						"/" + expectedContainerName,
					},
				},
			},
			nil,
		)
		fakeDockerClient.ContainerInspectReturns(
			types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						StartedAt: expectedStartedAt.Format(time.RFC3339Nano),
					},
				},
			},
			nil,
		)

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		actualContainers, actualErr := objectUnderTest.ListContainersByLabels(
			providedCtx,
			providedLabels,
		)

		/* assert */
		Expect(actualErr).To(BeNil())
		Expect(actualContainers).To(Equal([]containerruntime.Container{
			{
				ID:        expectedContainerID,
				Name:      expectedContainerName,
				Image:     expectedImage,
				StartedAt: expectedStartedAt,
				Labels:    expectedLabels,
			},
		}))

		actualCtx, actualContainerListOptions := fakeDockerClient.ContainerListArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerListOptions).To(Equal(expectedContainerListOptions))

		actualCtx, actualContainerName := fakeDockerClient.ContainerInspectArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerName).To(Equal(expectedContainerName))
	})

	It("should list every opctl container when labels are omitted", func() {
		/* arrange */
		providedCtx := context.Background()
		expectedContainerListOptions := container.ListOptions{
			All:     true,
			Filters: getOpctlContainerFilters(),
		}

		fakeDockerClient := new(FakeCommonAPIClient)
		fakeDockerClient.ContainerListReturns(
			[]types.Container{},
			nil,
		)

		objectUnderTest := _containerRuntime{
			dockerClient: fakeDockerClient,
		}

		/* act */
		actualContainers, actualErr := objectUnderTest.ListContainersByLabels(
			providedCtx,
			nil,
		)

		/* assert */
		Expect(actualErr).To(BeNil())
		Expect(actualContainers).To(BeEmpty())

		actualCtx, actualContainerListOptions := fakeDockerClient.ContainerListArgsForCall(0)
		Expect(actualCtx).To(Equal(providedCtx))
		Expect(actualContainerListOptions).To(Equal(expectedContainerListOptions))
	})
})
