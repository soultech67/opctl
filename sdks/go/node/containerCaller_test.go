package node

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/containerlog"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/fakes"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
)

var _ = Context("containerCaller", func() {
	dbDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}

	db, err := badger.Open(
		badger.DefaultOptions(dbDir).WithLogger(nil),
	)
	if err != nil {
		panic(err)
	}

	Context("newContainerCaller", func() {
		It("should return containerCaller", func() {
			/* arrange/act/assert */
			pubSub := pubsub.New(db)

			Expect(newContainerCaller(
				new(FakeContainerRuntime),
				pubSub,
				newStateStore(
					context.Background(),
					db,
					pubSub,
				),
				"",
			)).To(Not(BeNil()))
		})
	})
	Context("Call", func() {
		It("should call containerRuntime.RunContainer w/ expected args", func() {
			/* arrange */
			providedCtx := context.Background()
			providedContainerCall := &model.ContainerCall{
				BaseCall: model.BaseCall{},
				Image:    &model.ContainerCallImage{},
			}
			providedRootCallID := "providedRootCallID"
			fakeContainerRuntime := new(FakeContainerRuntime)

			fakeContainerRuntime.RunContainerStub = func(
				ctx context.Context,
				req *model.ContainerCall,
				rootCallID string,
				eventPublisher pubsub.EventPublisher,
				stdOut io.WriteCloser,
				stdErr io.WriteCloser,
			) (*int64, error) {

				stdErr.Close()
				stdOut.Close()

				return nil, nil
			}

			pubSub := pubsub.New(db)

			objectUnderTest := _containerCaller{
				containerRuntime: fakeContainerRuntime,
				pubSub:           pubSub,
			}

			/* act */
			objectUnderTest.Call(
				providedCtx,
				providedContainerCall,
				map[string]*model.Value{},
				&model.ContainerCallSpec{},
				providedRootCallID,
			)

			/* assert */
			_,
				actualContainerCall,
				actualRootCallID,
				actualEventPublisher,
				_,
				_ := fakeContainerRuntime.RunContainerArgsForCall(0)
			Expect(actualContainerCall).To(Equal(providedContainerCall))
			Expect(actualRootCallID).To(Equal(providedRootCallID))
			Expect(actualEventPublisher).To(Equal(pubSub))
		})
		Context("containerRuntime.RunContainer errors", func() {
			It("should publish expected ContainerExited", func() {
				/* arrange */
				expectedErrorMessage := "expectedErrorMessage"

				fakeContainerRuntime := new(FakeContainerRuntime)

				fakeContainerRuntime.RunContainerStub = func(
					ctx context.Context,
					req *model.ContainerCall,
					rootCallID string,
					eventPublisher pubsub.EventPublisher,
					stdOut io.WriteCloser,
					stdErr io.WriteCloser,
				) (*int64, error) {

					stdErr.Close()
					stdOut.Close()

					return nil, errors.New(expectedErrorMessage)
				}

				objectUnderTest := _containerCaller{
					containerRuntime: fakeContainerRuntime,
					pubSub:           pubsub.New(db),
				}

				/* act */
				actualOutputs, actualErr := objectUnderTest.Call(
					context.Background(),
					&model.ContainerCall{
						BaseCall: model.BaseCall{},
						Image:    &model.ContainerCallImage{},
					},
					map[string]*model.Value{},
					&model.ContainerCallSpec{},
					"rootCallID",
				)

				/* assert */
				Expect(actualOutputs).To(Equal(map[string]*model.Value{}))
				Expect(actualErr).To(MatchError(expectedErrorMessage))
			})
		})
	})

	Context("container.log persistence", func() {
		// default dir for the runWithOutput container (opPath "testop", no name).
		defaultDir := func(dataDirPath string) string {
			return containerlog.DefaultDir(dataDirPath, "testop", nil)
		}

		runWithOutput := func(dataDirPath string, log *model.ContainerLog) {
			fakeContainerRuntime := new(FakeContainerRuntime)
			fakeContainerRuntime.RunContainerStub = func(
				ctx context.Context,
				req *model.ContainerCall,
				rootCallID string,
				eventPublisher pubsub.EventPublisher,
				stdOut io.WriteCloser,
				stdErr io.WriteCloser,
			) (*int64, error) {
				io.WriteString(stdOut, "hello stdout\n")
				io.WriteString(stdErr, "hello stderr\n")
				stdOut.Close()
				stdErr.Close()
				return nil, nil
			}

			objectUnderTest := _containerCaller{
				containerRuntime: fakeContainerRuntime,
				pubSub:           pubsub.New(db),
				dataDirPath:      dataDirPath,
				logWriters:       &sync.Map{},
			}

			objectUnderTest.Call(
				context.Background(),
				&model.ContainerCall{
					BaseCall: model.BaseCall{OpPath: "testop"},
					Image:    &model.ContainerCallImage{},
					Log:      log,
				},
				map[string]*model.Value{},
				&model.ContainerCallSpec{},
				"rootCallID",
			)
		}

		It("persists stdout and stderr to separate files in the default location", func() {
			/* arrange */
			dataDir, dirErr := os.MkdirTemp("", "")
			Expect(dirErr).To(BeNil())
			enabled := true

			/* act */
			runWithOutput(dataDir, &model.ContainerLog{Enabled: &enabled})

			/* assert */
			outBytes, outErr := os.ReadFile(filepath.Join(defaultDir(dataDir), "stdout.log"))
			Expect(outErr).To(BeNil())
			Expect(string(outBytes)).To(Equal("hello stdout\n"))

			errBytes, errErr := os.ReadFile(filepath.Join(defaultDir(dataDir), "stderr.log"))
			Expect(errErr).To(BeNil())
			Expect(string(errBytes)).To(Equal("hello stderr\n"))
		})

		It("writes to a custom log.dir with name-prefixed files", func() {
			/* arrange */
			logDir, dirErr := os.MkdirTemp("", "")
			Expect(dirErr).To(BeNil())
			enabled := true

			/* act */
			runWithOutput("", &model.ContainerLog{Dir: logDir, Enabled: &enabled})

			/* assert (name unset -> "container" prefix) */
			outBytes, outErr := os.ReadFile(filepath.Join(logDir, "container.stdout.log"))
			Expect(outErr).To(BeNil())
			Expect(string(outBytes)).To(Equal("hello stdout\n"))
		})

		It("writes no files when disabled", func() {
			/* arrange */
			dataDir, dirErr := os.MkdirTemp("", "")
			Expect(dirErr).To(BeNil())
			disabled := false

			/* act */
			runWithOutput(dataDir, &model.ContainerLog{Enabled: &disabled})

			/* assert */
			_, statErr := os.Stat(filepath.Join(dataDir, "logs"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})

		It("is on by default when log config is absent", func() {
			/* arrange */
			os.Unsetenv("OPCTL_CONTAINER_LOG")
			dataDir, dirErr := os.MkdirTemp("", "")
			Expect(dirErr).To(BeNil())

			/* act */
			runWithOutput(dataDir, nil)

			/* assert */
			outBytes, outErr := os.ReadFile(filepath.Join(defaultDir(dataDir), "stdout.log"))
			Expect(outErr).To(BeNil())
			Expect(string(outBytes)).To(Equal("hello stdout\n"))
		})

		It("caches one rotating writer per log path across calls (no per-call goroutine leak)", func() {
			/* arrange */
			os.Unsetenv("OPCTL_CONTAINER_LOG")
			dataDir, dirErr := os.MkdirTemp("", "")
			Expect(dirErr).To(BeNil())
			enabled := true
			cache := &sync.Map{} // shared across calls (mirrors the singleton caller)

			call := func() {
				fakeContainerRuntime := new(FakeContainerRuntime)
				fakeContainerRuntime.RunContainerStub = func(
					ctx context.Context,
					req *model.ContainerCall,
					rootCallID string,
					eventPublisher pubsub.EventPublisher,
					stdOut io.WriteCloser,
					stdErr io.WriteCloser,
				) (*int64, error) {
					io.WriteString(stdOut, "x\n")
					stdOut.Close()
					stdErr.Close()
					return nil, nil
				}
				cc := _containerCaller{
					containerRuntime: fakeContainerRuntime,
					pubSub:           pubsub.New(db),
					dataDirPath:      dataDir,
					logWriters:       cache,
				}
				cc.Call(
					context.Background(),
					&model.ContainerCall{
						BaseCall: model.BaseCall{OpPath: "testop"},
						Image:    &model.ContainerCallImage{},
						Log:      &model.ContainerLog{Enabled: &enabled},
					},
					map[string]*model.Value{},
					&model.ContainerCallSpec{},
					"rootCallID",
				)
			}

			/* act: three calls to the same container (same log path) */
			call()
			call()
			call()

			/* assert: exactly 2 cached writers (stdout + stderr), not 6 */
			count := 0
			cache.Range(func(_, _ interface{}) bool {
				count++
				return true
			})
			Expect(count).To(Equal(2))
		})
	})

	It("should return expected results", func() {
		/* arrange */
		providedOpPath := "providedOpPath"
		providedContainerCall := &model.ContainerCall{
			BaseCall: model.BaseCall{
				OpPath: providedOpPath,
			},
			ContainerID: "providedContainerID",
			Image:       &model.ContainerCallImage{},
		}
		providedInboundScope := map[string]*model.Value{}
		providedContainerCallSpec := &model.ContainerCallSpec{}

		fakeContainerRuntime := new(FakeContainerRuntime)

		expectedErr := errors.New("io: read/write on closed pipe")
		fakeContainerRuntime.RunContainerStub = func(
			ctx context.Context,
			req *model.ContainerCall,
			rootCallID string,
			eventPublisher pubsub.EventPublisher,
			stdOut io.WriteCloser,
			stdErr io.WriteCloser,
		) (*int64, error) {

			stdErr.Close()
			stdOut.Close()

			return nil, expectedErr
		}

		objectUnderTest := _containerCaller{
			containerRuntime: fakeContainerRuntime,
			pubSub:           pubsub.New(db),
		}

		/* act */
		actualOutputs, actualErr := objectUnderTest.Call(
			context.Background(),
			providedContainerCall,
			providedInboundScope,
			providedContainerCallSpec,
			"rootCallID",
		)

		/* assert */
		Expect(actualOutputs).To(Equal(map[string]*model.Value{}))
		Expect(actualErr).To(Equal(expectedErr))
	})
})
