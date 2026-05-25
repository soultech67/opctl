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
	Context("RemoveAuth", func() {
		It("should publish an AuthRemoved event with the requested resources", func() {
			/* arrange */
			providedReq := model.RemoveAuthReq{Resources: "docker.io"}

			dbDir, err := os.MkdirTemp("", "removeAuth-test-*")
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

			objectUnderTest := core{pubSub: pubSub}

			/* act */
			Expect(objectUnderTest.RemoveAuth(context.Background(), providedReq)).To(Succeed())

			/* assert */
			var actualEvent model.Event
			go func() {
				for event := range eventChannel {
					if event.AuthRemoved != nil {
						actualEvent = event
					}
				}
			}()

			Eventually(func() *model.AuthRemoved {
				return actualEvent.AuthRemoved
			}).ShouldNot(BeNil())
			Expect(actualEvent.AuthRemoved.Resources).To(Equal(providedReq.Resources))
			Expect(actualEvent.Timestamp).NotTo(BeZero())
		})
	})
})
