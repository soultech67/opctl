package logs

import (
	"fmt"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/opspec/interpreter/dir"
)

// Interpret a container call's `log` block into the interpreted model: the
// opfile rotation overrides (nil where unspecified) plus, when log.dir is set,
// the resolved host directory.
//
// Returns nil when there is no `log` block — logging is still on by default,
// but default file-path computation + node-level/hardcoded defaulting happen in
// the node layer (sdks/go/node/containerlog.Resolve), which keeps this
// interpreter free of data-dir/env concerns and means containers without a
// `log` block interpret to an unchanged ContainerCall.
func Interpret(
	scope map[string]*model.Value,
	containerLogSpec *model.ContainerLogSpec,
	scratchDir string,
) (*model.ContainerLog, error) {
	if containerLogSpec == nil {
		return nil, nil
	}

	containerLog := &model.ContainerLog{
		Enabled:    containerLogSpec.Enabled,
		MaxSizeMB:  containerLogSpec.MaxSizeMB,
		MaxBackups: containerLogSpec.MaxBackups,
		MaxAgeDays: containerLogSpec.MaxAgeDays,
		Compress:   containerLogSpec.Compress,
	}

	if containerLogSpec.Dir != nil {
		// user-chosen host directory, resolved exactly like a dirs value.
		dirValue, err := dir.Interpret(scope, containerLogSpec.Dir, scratchDir, true)
		if err != nil {
			return nil, fmt.Errorf("unable to interpret container log dir: %w", err)
		}
		if dirValue.Dir != nil {
			containerLog.Dir = *dirValue.Dir
		}
	}

	return containerLog, nil
}
