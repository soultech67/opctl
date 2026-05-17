package docker

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("getContainerName", func() {
	It("should use the first readable hint and a short container ID", func() {
		/* arrange */
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"

		/* act */
		actualContainerName := getContainerName(
			providedContainerID,
			"Build / App!",
			"fallback",
		)

		/* assert */
		Expect(actualContainerName).To(Equal("opctl_build-app_2a647646"))
	})

	It("should fall back to the short container ID", func() {
		/* arrange */
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"

		/* act */
		actualContainerName := getContainerName(
			providedContainerID,
			"!!!",
		)

		/* assert */
		Expect(actualContainerName).To(Equal("opctl_2a647646"))
	})

	It("should truncate long readable hints", func() {
		/* arrange */
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"
		providedNameHint := strings.Repeat("a", containerNameSlugMaxLength+1)

		/* act */
		actualContainerName := getContainerName(
			providedContainerID,
			providedNameHint,
		)

		/* assert */
		Expect(actualContainerName).To(Equal("opctl_" + strings.Repeat("a", containerNameSlugMaxLength) + "_2a647646"))
	})
})

var _ = Context("getContainerNameForCall", func() {
	It("should prefer container name over image name", func() {
		/* arrange */
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"
		providedName := "Container Name"
		providedImageRef := "ghcr.io/opctl/image-name:latest"
		providedReq := &model.ContainerCall{
			ContainerID: providedContainerID,
			Image: &model.ContainerCallImage{
				Ref: &providedImageRef,
			},
			Name: &providedName,
		}

		/* act */
		actualContainerName := getContainerNameForCall(providedReq)

		/* assert */
		Expect(actualContainerName).To(Equal("opctl_container-name_2a647646"))
	})

	It("should fall back to image name", func() {
		/* arrange */
		providedContainerID := "2a647646e9cc4ef4940f52ad944f5657"
		providedImageRef := "ghcr.io/opctl/image-name:latest"
		providedReq := &model.ContainerCall{
			ContainerID: providedContainerID,
			Image: &model.ContainerCallImage{
				Ref: &providedImageRef,
			},
		}

		/* act */
		actualContainerName := getContainerNameForCall(providedReq)

		/* assert */
		Expect(actualContainerName).To(Equal("opctl_image-name_2a647646"))
	})
})
