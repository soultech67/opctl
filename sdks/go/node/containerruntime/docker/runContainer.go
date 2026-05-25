package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerClientPkg "github.com/docker/docker/client"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/dns"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
	"golang.org/x/sync/errgroup"
)

type runContainer interface {
	RunContainer(
		ctx context.Context,
		req *model.ContainerCall,
		rootCallID string,
		eventPublisher pubsub.EventPublisher,
		stdout io.WriteCloser,
		stderr io.WriteCloser,
	) (*int64, error)
}

func newRunContainer(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
) (runContainer, error) {
	rc := _runContainer{
		containerStdErrStreamer: newContainerStdErrStreamer(dockerClient),
		containerStdOutStreamer: newContainerStdOutStreamer(dockerClient),
		dockerClient:            dockerClient,
	}
	return rc, nil
}

type _runContainer struct {
	containerStdErrStreamer containerLogStreamer
	containerStdOutStreamer containerLogStreamer
	dockerClient            dockerClientPkg.CommonAPIClient
}

func (cr _runContainer) RunContainer(
	ctx context.Context,
	req *model.ContainerCall,
	rootCallID string,
	eventPublisher pubsub.EventPublisher,
	stdout io.WriteCloser,
	stderr io.WriteCloser,
) (*int64, error) {
	defer stdout.Close()
	defer stderr.Close()

	// Probe Docker once at op start. A wedged daemon fails fast here with a
	// clear error event instead of blocking inside ContainerCreate below.
	if err := pingDocker(ctx, cr.dockerClient); err != nil {
		return nil, fmt.Errorf("docker daemon not responding: %w; try `docker info` or restart Docker Desktop", err)
	}

	// ensure user defined network exists to allow inter container resolution via name
	// @TODO: remove when socket outputs supported
	if err := ensureNetworkExists(
		ctx,
		cr.dockerClient,
		networkName,
	); err != nil {
		return nil, err
	}

	// for docker, we prefix name with opctl_ in order to allow external tools to know it's an opctl managed container
	// do not change this prefix as it might break external consumers
	dockerContainerName := getContainerNameForCall(req)
	defer func() {
		// Ensure container is always cleaned up: gracefully stop, then delete.
		// Critical: this runs on a FRESH context (parent may be cancelled), but
		// it MUST be bounded — without a deadline, a wedged Docker leaves this
		// goroutine blocked forever, no CallEnded event ever fires, and the CLI
		// hangs on Ctrl+C. Bounded by dockerCleanupTimeout; if we exceed it we
		// surface a warning event so the user sees the smoking-gun message
		// rather than silent hang.
		cleanupCtx, cancel := withDockerTimeout(context.Background(), dockerCleanupTimeout())
		defer cancel()

		dockerInstrInfof("cleanup starting: container=%s", dockerContainerName)
		cleanupStart := time.Now()
		cleanupErr := deleteContainer(cleanupCtx, cr.dockerClient, dockerContainerName)
		cleanupElapsed := time.Since(cleanupStart)

		switch {
		case cleanupErr == nil:
			dockerInstrInfof("cleanup ok in %s: container=%s", cleanupElapsed, dockerContainerName)
		case isDeadlineExceeded(cleanupErr):
			dockerInstrInfof("cleanup TIMED OUT after %s: container=%s err=%v", cleanupElapsed, dockerContainerName, cleanupErr)
			publishCleanupTimeoutWarning(eventPublisher, req, rootCallID, dockerContainerName, cleanupElapsed)
		default:
			dockerInstrInfof("cleanup failed after %s: container=%s err=%v", cleanupElapsed, dockerContainerName, cleanupErr)
		}
	}()

	var imageErr error
	if req.Image.Src != nil {
		imageRef := fmt.Sprintf("%s:latest", req.ContainerID)
		req.Image.Ref = &imageRef

		imageErr = pushImage(
			ctx,
			imageRef,
			req.Image.Src,
		)
	} else {
		imageErr = pullImage(
			ctx,
			req,
			cr.dockerClient,
			rootCallID,
			eventPublisher,
		)
		// don't err yet; image might be cached. We allow this to support offline use
	}

	portBindings, err := constructPortBindings(
		req.Ports,
	)
	if err != nil {
		return nil, err
	}

	// construct networking config
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}
	if req.Name != nil {
		networkingConfig.EndpointsConfig[networkName].Aliases = []string{
			*req.Name,
		}
	}

	isGpuSupported, err := isGpuSupported(ctx, cr.dockerClient, req.Image.PullCreds)
	if nil != err {
		// Failure to determine GPU support just really means no, GPU is not supported.
		isGpuSupported = false
	}

	// create container
	createCtx, cancelCreate := withDockerTimeout(ctx, dockerMutationTimeout())
	createErr := instrumentedDockerCall("ContainerCreate", dockerContainerName, func() error {
		_, err := cr.dockerClient.ContainerCreate(
			createCtx,
			constructContainerConfig(
				req.Cmd,
				req.EnvVars,
				*req.Image.Ref,
				portBindings,
				req.WorkDir,
				getContainerLabelsForCall(req),
			),
			constructHostConfig(
				req.Dirs,
				req.Files,
				req.Sockets,
				portBindings,
				isGpuSupported,
			),
			networkingConfig,
			// platform requires API v1.41 so set to nil to avoid version errors
			nil,
			dockerContainerName,
		)
		return err
	})
	cancelCreate()
	if createErr != nil {
		select {
		case <-ctx.Done():
			// we got killed;
			return nil, nil
		default:
			return nil, errors.Join(imageErr, createErr)
		}
	}

	// start container
	startCtx, cancelStart := withDockerTimeout(ctx, dockerMutationTimeout())
	startErr := instrumentedDockerCall("ContainerStart", dockerContainerName, func() error {
		return cr.dockerClient.ContainerStart(
			startCtx,
			dockerContainerName,
			container.StartOptions{},
		)
	})
	cancelStart()
	if startErr != nil {
		return nil, startErr
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(
		func() error {
			return cr.containerStdErrStreamer.Stream(
				ctx,
				dockerContainerName,
				stderr,
			)
		},
	)
	eg.Go(
		func() error {
			return cr.containerStdOutStreamer.Stream(
				ctx,
				dockerContainerName,
				stdout,
			)
		},
	)

	if req.Name != nil {
		inspectCtx, cancelInspect := withDockerTimeout(ctx, dockerInspectTimeout())
		containerJSON, err := func() (types.ContainerJSON, error) {
			defer cancelInspect()
			var cj types.ContainerJSON
			inspectErr := instrumentedDockerCall("ContainerInspect", "dns lookup "+dockerContainerName, func() error {
				var innerErr error
				cj, innerErr = cr.dockerClient.ContainerInspect(inspectCtx, dockerContainerName)
				return innerErr
			})
			return cj, inspectErr
		}()
		if err != nil {
			return nil, err
		}

		if endpointSettings, ok := containerJSON.NetworkSettings.Networks[networkName]; ok {
			defer dns.UnregisterName(
				*req.Name,
				endpointSettings.IPAddress,
			)

			err = dns.RegisterName(
				ctx,
				*req.Name,
				endpointSettings.IPAddress,
			)
			if err != nil {
				return nil, err
			}
		}
	}

	var (
		exitCode int64
		waitErr  error
	)
	waitOkChan, waitErrChan := cr.dockerClient.ContainerWait(
		ctx,
		dockerContainerName,
		container.WaitConditionNotRunning,
	)
	select {
	case waitOk := <-waitOkChan:
		exitCode = waitOk.StatusCode
	case waitErr := <-waitErrChan:
		waitErr = fmt.Errorf("error waiting on container: %w", waitErr)
	}

	// ensure stdout, and stderr all read before returning
	logErr := eg.Wait()

	return &exitCode, errors.Join(waitErr, logErr)

}

// publishCleanupTimeoutWarning emits a ContainerStdErrWrittenTo event so the
// CLI surfaces a smoking-gun message when the deferred cleanup blocks past its
// budget. The message names the timeout and points the user at the recovery
// path (restart Docker). This is the user-visible counterpart to the
// dockerInstrInfof log line.
func publishCleanupTimeoutWarning(
	eventPublisher pubsub.EventPublisher,
	req *model.ContainerCall,
	rootCallID string,
	dockerContainerName string,
	elapsed time.Duration,
) {
	if eventPublisher == nil || req == nil {
		return
	}

	msg := fmt.Sprintf(
		"\nwarning: cleanup of container %s timed out after %s — Docker may be unresponsive. Try `docker info` or restart Docker Desktop.\n",
		dockerContainerName,
		elapsed,
	)
	eventPublisher.Publish(model.Event{
		Timestamp: time.Now().UTC(),
		ContainerStdErrWrittenTo: &model.ContainerStdErrWrittenTo{
			Data:        []byte(msg),
			OpRef:       req.OpPath,
			ContainerID: req.ContainerID,
			RootCallID:  rootCallID,
		},
	})
}
