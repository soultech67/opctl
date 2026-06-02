package node

import (
	"context"
	"os"

	"github.com/dgraph-io/badger/v4"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
)

var _ = Context("core", func() {
	Context("AddAuth", func() {
		It("should publish an AuthAdded event with the requested creds + resources", func() {
			/* arrange */
			providedReq := model.AddAuthReq{
				Creds: model.Creds{
					Username: "username",
					Password: "password",
				},
				Resources: "resources",
			}

			dbDir, err := os.MkdirTemp("", "addAuth-test-*")
			if err != nil {
				panic(err)
			}
			db, err := badger.Open(badger.DefaultOptions(dbDir).WithLogger(nil))
			if err != nil {
				panic(err)
			}

			pubSub := pubsub.New(db)
			eventChannel, err := pubSub.Subscribe(context.Background(), model.EventFilter{})
			if err != nil {
				panic(err)
			}

			// AddAuth now waits for the AuthAdded event to be durably applied
			// before returning, so the core needs a real stateStore subscribed
			// to this pubSub (the db-backed pubsub replays the event, so there's
			// no subscribe-vs-publish race).
			ss := newStateStore(context.Background(), db, pubSub)
			objectUnderTest := core{pubSub: pubSub, stateStore: ss}

			/* act */
			Expect(objectUnderTest.AddAuth(context.Background(), providedReq)).To(Succeed())

			/* assert */
			var actualEvent model.Event
			go func() {
				for event := range eventChannel {
					if event.AuthAdded != nil {
						actualEvent = event
					}
				}
			}()

			Eventually(func() *model.AuthAdded {
				return actualEvent.AuthAdded
			}).ShouldNot(BeNil())
			Expect(actualEvent.AuthAdded.Auth).To(Equal(model.Auth{
				Creds:     providedReq.Creds,
				Resources: providedReq.Resources,
			}))
			Expect(actualEvent.Timestamp).NotTo(BeZero())
		})
		It("should result in the auth being retrievable via TryGetAuth once applied", func() {
			/* arrange: skip the pubsub timing dance by applying the event to the store directly */
			ss := newTestStateStore()
			providedReq := model.AddAuthReq{
				Creds: model.Creds{
					Username: "username",
					Password: "password",
				},
				Resources: "docker.io",
			}

			/* act */
			Expect(ss.applyAuthAdded(model.AuthAdded{
				Auth: model.Auth{
					Creds:     providedReq.Creds,
					Resources: providedReq.Resources,
				},
			})).To(Succeed())

			/* assert */
			actual := ss.TryGetAuth(providedReq.Resources)
			Expect(actual).NotTo(BeNil())
			Expect(actual.Username).To(Equal(providedReq.Username))
			Expect(actual.Password).To(Equal(providedReq.Password))
			Expect(actual.Resources).To(Equal(providedReq.Resources))
		})
	})
})
