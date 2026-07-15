package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
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
	//
	// IMPORTANT: createCtx is derived from context.Background(), NOT the
	// parent ctx. Docker's apiproxy cannot abort an in-flight ContainerCreate
	// when the client disconnects — dockerd may continue processing the
	// request server-side and leave a `Created`-state container with all the
	// bind mounts the request asked for, invisible in `docker ps` and silently
	// blocking subsequent ContainerCreate calls that want overlapping mounts.
	// (Empirically confirmed via Docker Desktop's VM init.log: cancelled
	// creates produce no `<<` response line, and ghost containers have been
	// observed after Ctrl+C on long-running ops.)
	//
	// By detaching from parent ctx we wait for dockerd to actually finish
	// (or for our own 20s timeout to fire), so we know definitively whether
	// a container exists. Cost: Ctrl+C may take up to dockerMutationTimeout
	// to take effect when a create is in flight; in exchange we never leak
	// orphan `Created`-state containers.
	var createdContainerID string
	createCtx, cancelCreate := withDockerTimeout(context.Background(), dockerMutationTimeout())
	createErr := instrumentedDockerCall("ContainerCreate", dockerContainerName, func() error {
		resp, err := cr.dockerClient.ContainerCreate(
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
				req.Volumes,
				portBindings,
				isGpuSupported,
			),
			networkingConfig,
			// platform requires API v1.41 so set to nil to avoid version errors
			nil,
			dockerContainerName,
		)
		if err == nil {
			createdContainerID = resp.ID
		}
		return err
	})
	cancelCreate()
	if createErr != nil {
		// Reconciliation: did dockerd finish the create despite our timeout?
		// If our 20s expired but dockerd kept going on its side, the container
		// may exist now (and would otherwise become an invisible orphan). Look
		// for it by the opctl.container-id label that getContainerLabelsForCall
		// stamps on every container we create, and if found, kill it.
		reconcileTimedOutContainerCreate(cr.dockerClient, req, dockerContainerName)

		select {
		case <-ctx.Done():
			// we got killed;
			return nil, nil
		default:
			return nil, errors.Join(imageErr, createErr)
		}
	}
	dockerInstrInfof("ContainerCreate created id=%s name=%s", createdContainerID, dockerContainerName)

	// If parent ctx was cancelled while we waited for ContainerCreate to
	// finish, the container now exists but the caller has given up. Return
	// cleanly; the deferred cleanup at the top of this function will
	// Stop+Remove it via its bounded fresh context.
	select {
	case <-ctx.Done():
		return nil, nil
	default:
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

// reconcileTimedOutContainerCreate handles the race where our ContainerCreate
// timed out but dockerd may have completed the create on its server side.
// We look for a container carrying our `opctl.container-id=<callID>` label
// (the label we stamp on every container we create via
// getContainerLabelsForCall) and, if found, force-remove it. Logs everything
// it does so the daemon log shows when we caught an orphan vs. when there
// was nothing there.
//
// Bounded on its own fresh context (does not inherit cancellation) and short
// timeout — we don't want to block returning from RunContainer if Docker is
// still wedged.
func reconcileTimedOutContainerCreate(
	dockerClient dockerClientPkg.CommonAPIClient,
	req *model.ContainerCall,
	dockerContainerName string,
) {
	if req == nil || req.ContainerID == "" {
		return
	}

	reconcileCtx, cancel := withDockerTimeout(context.Background(), dockerInspectTimeout())
	defer cancel()

	containerIDLabelFilter := getContainerIDLabelFilter(req.ContainerID)
	dockerInstrInfof("ContainerCreate-reconcile: searching for orphan with label %s", containerIDLabelFilter)

	var found []types.Container
	err := instrumentedDockerCall("ContainerList", "ContainerCreate-reconcile "+dockerContainerName, func() error {
		var listErr error
		found, listErr = dockerClient.ContainerList(reconcileCtx, container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.KeyValuePair{Key: "label", Value: containerIDLabelFilter},
			),
		})
		return listErr
	})
	if err != nil {
		dockerInstrInfof("ContainerCreate-reconcile: ContainerList failed (%v) — orphan may still exist; `opctl container prune` will clean it up", err)
		return
	}

	if len(found) == 0 {
		dockerInstrInfof("ContainerCreate-reconcile: no orphan found; dockerd really did not create the container")
		return
	}

	for _, c := range found {
		dockerInstrInfof("ContainerCreate-reconcile: FOUND orphan id=%s names=%v state=%s — killing", c.ID, c.Names, c.State)
		killCtx, killCancel := withDockerTimeout(context.Background(), dockerMutationTimeout())
		killErr := instrumentedDockerCall("ContainerRemove", "ContainerCreate-reconcile "+c.ID, func() error {
			return dockerClient.ContainerRemove(killCtx, c.ID, container.RemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			})
		})
		killCancel()
		if killErr != nil && !isContainerDeleteAlreadyDone(killErr) {
			dockerInstrInfof("ContainerCreate-reconcile: failed to remove orphan id=%s: %v (`opctl container prune` will retry)", c.ID, killErr)
		}
	}
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
