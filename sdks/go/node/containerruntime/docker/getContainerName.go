package docker

import (
	"fmt"
	"path"
	"strings"

	"github.com/opctl/opctl/sdks/go/model"
)

const containerNameIDLength = 8
const containerNameSlugMaxLength = 63

func getContainerName(containerID string, nameHints ...string) string {
	slug := getContainerNameSlug(nameHints...)
	id := getContainerNameID(containerID)
	if slug == "" {
		return fmt.Sprintf("%s%s", containerNamePrefix, id)
	}
	if id == "" {
		return fmt.Sprintf("%s%s", containerNamePrefix, slug)
	}
	return fmt.Sprintf("%s%s_%s", containerNamePrefix, slug, id)
}

func getContainerNameForCall(req *model.ContainerCall) string {
	nameHints := []string{}
	if req.Name != nil {
		nameHints = append(nameHints, *req.Name)
	}
	if req.Image != nil && req.Image.Ref != nil {
		nameHints = append(nameHints, getImageNameHint(*req.Image.Ref))
	}

	return getContainerName(req.ContainerID, nameHints...)
}

func getLegacyContainerName(containerID string) string {
	return fmt.Sprintf("%s%s", containerNamePrefix, containerID)
}

func isOpctlContainerName(containerName string) bool {
	return strings.HasPrefix(normalizeDockerContainerName(containerName), containerNamePrefix)
}

func normalizeDockerContainerName(containerName string) string {
	return strings.TrimPrefix(containerName, "/")
}

func getContainerNameID(containerID string) string {
	if len(containerID) <= containerNameIDLength {
		return containerID
	}
	return containerID[:containerNameIDLength]
}

func getContainerNameSlug(nameHints ...string) string {
	for _, nameHint := range nameHints {
		slug := slugifyContainerNameHint(nameHint)
		if slug != "" {
			return slug
		}
	}
	return ""
}

func slugifyContainerNameHint(nameHint string) string {
	var slug strings.Builder
	previousWasSeparator := false

	for _, char := range strings.ToLower(strings.TrimSpace(nameHint)) {
		switch {
		case char >= 'a' && char <= 'z':
			slug.WriteRune(char)
			previousWasSeparator = false
		case char >= '0' && char <= '9':
			slug.WriteRune(char)
			previousWasSeparator = false
		default:
			if slug.Len() > 0 && !previousWasSeparator {
				slug.WriteRune('-')
				previousWasSeparator = true
			}
		}
	}

	result := strings.Trim(slug.String(), "-")
	if len(result) > containerNameSlugMaxLength {
		result = strings.Trim(result[:containerNameSlugMaxLength], "-")
	}
	return result
}

func getImageNameHint(imageRef string) string {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return ""
	}

	if digestIndex := strings.Index(imageRef, "@"); digestIndex >= 0 {
		imageRef = imageRef[:digestIndex]
	}

	imageName := path.Base(imageRef)
	if tagIndex := strings.LastIndex(imageName, ":"); tagIndex >= 0 {
		imageName = imageName[:tagIndex]
	}

	return imageName
}
