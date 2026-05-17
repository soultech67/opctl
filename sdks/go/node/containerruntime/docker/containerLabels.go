package docker

import (
	"fmt"

	"github.com/opctl/opctl/sdks/go/model"
)

const containerIDLabelName = "opctl.container-id"
const containerNameLabelName = "opctl.container-name"
const imageRefLabelName = "opctl.image-ref"
const managedContainerLabelName = "opctl.managed"
const managedContainerLabelValue = "true"

func getContainerLabelsForCall(req *model.ContainerCall) map[string]string {
	labels := map[string]string{
		containerIDLabelName:      req.ContainerID,
		managedContainerLabelName: managedContainerLabelValue,
	}
	if req.Name != nil {
		labels[containerNameLabelName] = *req.Name
	}
	if req.Image != nil && req.Image.Ref != nil {
		labels[imageRefLabelName] = *req.Image.Ref
	}

	return labels
}

func getContainerIDLabelFilter(containerID string) string {
	return fmt.Sprintf("%s=%s", containerIDLabelName, containerID)
}

func getManagedContainerLabelFilter() string {
	return fmt.Sprintf("%s=%s", managedContainerLabelName, managedContainerLabelValue)
}
