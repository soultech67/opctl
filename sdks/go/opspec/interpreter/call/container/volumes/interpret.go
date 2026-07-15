package volumes

import (
	"fmt"
	"regexp"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/opspec/interpreter/str"
)

// mirrors the docker daemon's restricted-name pattern for named volumes, so op
// authors get an interpret-time error instead of a raw container-create error.
var volumeNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// unix absolute path or Windows drive-prefixed path; the container runtime
// rejects relative mount targets, so catch them at interpret time too
var absolutePathRegexp = regexp.MustCompile(`^(/|[a-zA-Z]:)`)

// Interpret container volumes
func Interpret(
	scope map[string]*model.Value,
	containerCallSpecVolumes map[string]string,
) (map[string]string, error) {
	containerCallVolumes := map[string]string{}
	for callSpecContainerVolumePath, volumeNameExpression := range containerCallSpecVolumes {
		if !absolutePathRegexp.MatchString(callSpecContainerVolumePath) {
			return nil, fmt.Errorf("unable to bind volume %v to %v: container path must be absolute", callSpecContainerVolumePath, volumeNameExpression)
		}

		volumeName, err := str.Interpret(
			scope,
			volumeNameExpression,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to bind volume %v to %v: %w", callSpecContainerVolumePath, volumeNameExpression, err)
		}

		interpretedName := ""
		if volumeName.String != nil {
			interpretedName = *volumeName.String
		}
		if !volumeNameRegexp.MatchString(interpretedName) {
			return nil, fmt.Errorf(
				"unable to bind volume %v to %v: %q isn't a valid volume name; must match %v",
				callSpecContainerVolumePath,
				volumeNameExpression,
				interpretedName,
				volumeNameRegexp.String(),
			)
		}

		containerCallVolumes[callSpecContainerVolumePath] = interpretedName
	}
	return containerCallVolumes, nil
}
