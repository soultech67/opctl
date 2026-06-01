package node

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/opctl/opctl/sdks/go/internal/unsudo"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/containerlog"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
	"github.com/opctl/opctl/sdks/go/opspec"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

//counterfeiter:generate -o internal/fakes/containerCaller.go . containerCaller
type containerCaller interface {
	// Executes a container call
	Call(
		ctx context.Context,
		containerCall *model.ContainerCall,
		inboundScope map[string]*model.Value,
		containerCallSpec *model.ContainerCallSpec,
		rootCallID string,
	) (
		map[string]*model.Value,
		error,
	)
}

func newContainerCaller(
	containerRuntime containerruntime.ContainerRuntime,
	pubSub pubsub.PubSub,
	stateStore stateStore,
	dataDirPath string,
) containerCaller {

	return _containerCaller{
		containerRuntime: containerRuntime,
		pubSub:           pubSub,
		stateStore:       stateStore,
		dataDirPath:      dataDirPath,
		logWriters:       &sync.Map{},
	}

}

type _containerCaller struct {
	containerRuntime containerruntime.ContainerRuntime
	pubSub           pubsub.PubSub
	stateStore       stateStore
	dataDirPath      string
	// logWriters caches per-log-file rotating writers (path -> *lumberjack.Logger)
	// for the daemon's lifetime. lumberjack starts a background mill goroutine per
	// writer that Close() does not reap, so a fresh writer per call would leak
	// goroutines; caching bounds them to the number of distinct log targets.
	logWriters *sync.Map
}

func (cc _containerCaller) Call(
	ctx context.Context,
	containerCall *model.ContainerCall,
	inboundScope map[string]*model.Value,
	containerCallSpec *model.ContainerCallSpec,
	rootCallID string,
) (
	map[string]*model.Value,
	error,
) {
	outputs := map[string]*model.Value{}
	var exitCode int64

	if containerCall.Image.Ref != nil && containerCall.Image.PullCreds == nil {
		if auth := cc.stateStore.TryGetAuth(*containerCall.Image.Ref); auth != nil {
			containerCall.Image.PullCreds = &auth.Creds
		}
	}

	logStdOutPR, logStdOutPW := io.Pipe()
	logStdErrPR, logStdErrPW := io.Pipe()

	// interpret logs
	logChan := make(chan error, 1)
	go func() {
		logChan <- cc.interpretLogs(
			logStdOutPR,
			logStdErrPR,
			containerCall,
			rootCallID,
		)
	}()

	outputs = cc.interpretOutputs(
		containerCallSpec,
		containerCall,
	)

	imageRef := ""
	if containerCall.Image.Ref != nil {
		imageRef = *containerCall.Image.Ref
	}

	slog.Debug("container call starting",
		"containerID", containerCall.ContainerID, "image", imageRef,
		"opRef", containerCall.OpPath, "rootCallID", rootCallID)

	runStartedAt := time.Now()
	rawExitCode, err := cc.containerRuntime.RunContainer(
		ctx,
		containerCall,
		rootCallID,
		cc.pubSub,
		logStdOutPW,
		logStdErrPW,
	)
	runDuration := time.Since(runStartedAt)

	// @TODO: handle no exit code
	if rawExitCode != nil {
		exitCode = *rawExitCode
	}

	if exitCode != 0 {
		err = fmt.Errorf("nonzero container exit code: %d", exitCode)
	}

	if err != nil {
		slog.Warn("container call ended",
			"containerID", containerCall.ContainerID, "image", imageRef,
			"opRef", containerCall.OpPath, "rootCallID", rootCallID,
			"exitCode", exitCode, "exitCodeKnown", rawExitCode != nil,
			"duration", runDuration.String(), "error", err.Error())
	} else {
		slog.Debug("container call ended",
			"containerID", containerCall.ContainerID, "image", imageRef,
			"opRef", containerCall.OpPath, "rootCallID", rootCallID,
			"exitCode", exitCode, "duration", runDuration.String())
	}

	// wait on logChan
	if logChanErr := <-logChan; err == nil {
		// non-destructively set err
		err = logChanErr
	}

	return outputs, err
}

func (this _containerCaller) interpretLogs(
	stdOutReader io.Reader,
	stdErrReader io.Reader,
	containerCall *model.ContainerCall,
	rootCallID string,
) error {
	// Persist the container's stdout/stderr to durable rotating files in
	// addition to the event stream, so logs are explorable after shutdown.
	// Additive + best-effort: a file error never affects the event stream or the
	// op. nil writers => persistence disabled.
	stdOutFile, stdErrFile := this.openContainerLogFiles(containerCall)
	// Writers are cached + reused for the daemon's lifetime and intentionally NOT
	// closed here (lumberjack's Close doesn't reap its mill goroutine). Just make
	// the active files user-readable — the daemon runs as root; best-effort.
	defer func() {
		for _, f := range []*lumberjack.Logger{stdOutFile, stdErrFile} {
			if f != nil {
				_ = unsudo.EnsureOwnership(f.Filename)
			}
		}
	}()

	stdOutLogChan := make(chan error, 1)
	go func() {
		// interpret stdOut
		stdOutLogChan <- readChunks(
			stdOutReader,
			func(chunk []byte) {
				this.pubSub.Publish(
					model.Event{
						Timestamp: time.Now().UTC(),
						ContainerStdOutWrittenTo: &model.ContainerStdOutWrittenTo{
							Data:        chunk,
							ContainerID: containerCall.ContainerID,
							OpRef:       containerCall.OpPath,
							RootCallID:  rootCallID,
						},
					},
				)
				if stdOutFile != nil {
					stdOutFile.Write(chunk)
				}
			})
	}()

	stdErrLogChan := make(chan error, 1)
	go func() {
		// interpret stdErr
		stdErrLogChan <- readChunks(
			stdErrReader,
			func(chunk []byte) {
				this.pubSub.Publish(
					model.Event{
						Timestamp: time.Now().UTC(),
						ContainerStdErrWrittenTo: &model.ContainerStdErrWrittenTo{
							Data:        chunk,
							ContainerID: containerCall.ContainerID,
							OpRef:       containerCall.OpPath,
							RootCallID:  rootCallID,
						},
					},
				)
				if stdErrFile != nil {
					stdErrFile.Write(chunk)
				}
			})
	}()

	// wait on logs
	stdOutLogErr := <-stdOutLogChan
	stdErrLogErr := <-stdErrLogChan

	// return errs
	if stdOutLogErr != nil {
		return stdOutLogErr
	}
	if stdErrLogErr != nil {
		return stdErrLogErr
	}

	return nil
}

// openContainerLogFiles returns per-stream rotating writers when container log
// persistence is enabled, else nil writers. Best-effort: any setup failure
// disables file logging without affecting the op (the event stream is unchanged).
func (cc _containerCaller) openContainerLogFiles(
	containerCall *model.ContainerCall,
) (stdOut, stdErr *lumberjack.Logger) {
	cfg := containerlog.Resolve(
		containerCall.Log,
		cc.dataDirPath,
		containerCall.OpPath,
		containerCall.Name,
	)
	if !cfg.Enabled {
		return nil, nil
	}

	// user-owned dir (daemon runs as root)
	if err := unsudo.CreateDir(filepath.Dir(cfg.StdOutPath)); err != nil {
		slog.Warn("container log: unable to create log dir; file logging disabled",
			"dir", filepath.Dir(cfg.StdOutPath), "error", err.Error())
		return nil, nil
	}

	return cc.cachedWriter(cfg, cfg.StdOutPath), cc.cachedWriter(cfg, cfg.StdErrPath)
}

// cachedWriter returns the shared rotating writer for path, creating it at most
// once per path (kept for the daemon's lifetime — see logWriters). Concurrent
// creators race via LoadOrStore; the loser's writer is discarded unwritten, so
// its lazy mill goroutine never starts.
func (cc _containerCaller) cachedWriter(cfg containerlog.Config, path string) *lumberjack.Logger {
	newWriter := func() *lumberjack.Logger {
		return &lumberjack.Logger{
			Filename:   path,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
		}
	}
	if cc.logWriters == nil {
		// no cache (e.g. a bare test instance) — acceptable in a short-lived process.
		return newWriter()
	}
	if existing, ok := cc.logWriters.Load(path); ok {
		return existing.(*lumberjack.Logger)
	}
	actual, _ := cc.logWriters.LoadOrStore(path, newWriter())
	return actual.(*lumberjack.Logger)
}

func (this _containerCaller) interpretOutputs(
	containerCallSpec *model.ContainerCallSpec,
	containerCall *model.ContainerCall,
) map[string]*model.Value {
	outputs := map[string]*model.Value{}

	for socketAddr, name := range containerCallSpec.Sockets {
		// add socket outputs
		if "0.0.0.0" == socketAddr {
			outputs[name] = &model.Value{Socket: &containerCall.ContainerID}
		}
	}
	for callSpecContainerFilePath, mountSrc := range containerCallSpec.Files {
		mountSrcStr, ok := mountSrc.(string)
		if !ok {
			continue
		}

		if mountSrcStr == "" {
			// skip embedded files
			continue
		}

		// add file outputs
		for callContainerFilePath, callHostFilePath := range containerCall.Files {
			if callSpecContainerFilePath == callContainerFilePath {
				// copy callHostFilePath before taking address; range vars have same address for every iteration
				value := callHostFilePath
				outputs[opspec.RefToName(mountSrcStr)] = &model.Value{File: &value}
			}
		}
	}
	for callSpecContainerDirPath, mountSrc := range containerCallSpec.Dirs {
		mountSrcStr, ok := mountSrc.(string)
		if !ok {
			continue
		}

		if mountSrcStr == "" {
			// skip embedded dirs
			continue
		}

		// add dir outputs
		for callContainerDirPath, callHostDirPath := range containerCall.Dirs {
			if callSpecContainerDirPath == callContainerDirPath {
				// copy callHostDirPath before taking address; range vars have same address for every iteration
				value := callHostDirPath
				outputs[opspec.RefToName(mountSrcStr)] = &model.Value{Dir: &value}
			}
		}
	}

	return outputs
}
