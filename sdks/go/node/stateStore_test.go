package node

import (
	"os"

	"github.com/dgraph-io/badger/v4"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

// newTestStateStore returns a _stateStore wired to a fresh on-disk badger DB,
// without the background event-application goroutine. Tests call the
// apply* methods directly so behavior is synchronous and deterministic.
func newTestStateStore() *_stateStore {
	dbDir, err := os.MkdirTemp("", "stateStore-test-*")
	if err != nil {
		panic(err)
	}
	db, err := badger.Open(badger.DefaultOptions(dbDir).WithLogger(nil))
	if err != nil {
		panic(err)
	}
	return &_stateStore{
		authsByResourcesKeyPrefix:    "authsByResources_",
		callsByID:                    map[string]*model.Call{},
		db:                           db,
		lastAppliedEventTimestampKey: "lastAppliedEventTimestamp",
	}
}

var _ = Context("stateStore", func() {
	Context("TryGetAuth", func() {
		Context("after applyAuthAdded", func() {
			It("should return the stored auth for an exact resources match", func() {
				/* arrange */
				ss := newTestStateStore()
				expectedAuth := model.Auth{
					Creds:     model.Creds{Username: "user", Password: "pw"},
					Resources: "docker.io",
				}
				Expect(ss.applyAuthAdded(model.AuthAdded{Auth: expectedAuth})).To(Succeed())

				/* act */
				actualAuth := ss.TryGetAuth("docker.io")

				/* assert */
				Expect(actualAuth).NotTo(BeNil())
				Expect(*actualAuth).To(Equal(expectedAuth))
			})
			It("should return the stored auth for a ref whose prefix matches", func() {
				/* arrange */
				ss := newTestStateStore()
				stored := model.Auth{
					Creds:     model.Creds{Username: "user", Password: "pw"},
					Resources: "docker.io",
				}
				Expect(ss.applyAuthAdded(model.AuthAdded{Auth: stored})).To(Succeed())

				/* act */
				actualAuth := ss.TryGetAuth("docker.io/library/python:3.12")

				/* assert */
				Expect(actualAuth).NotTo(BeNil())
				Expect(*actualAuth).To(Equal(stored))
			})
			It("should match case-insensitively on the ref", func() {
				/* arrange */
				ss := newTestStateStore()
				stored := model.Auth{
					Creds:     model.Creds{Username: "user", Password: "pw"},
					Resources: "docker.io",
				}
				Expect(ss.applyAuthAdded(model.AuthAdded{Auth: stored})).To(Succeed())

				/* act */
				actualAuth := ss.TryGetAuth("DOCKER.IO/Library/Python")

				/* assert */
				Expect(actualAuth).NotTo(BeNil())
				Expect(*actualAuth).To(Equal(stored))
			})
		})
		Context("no matching auth stored", func() {
			It("should return nil", func() {
				/* arrange */
				ss := newTestStateStore()

				/* act / assert */
				Expect(ss.TryGetAuth("docker.io")).To(BeNil())
			})
		})
		Context("after applyAuthRemoved", func() {
			It("should no longer return the removed auth", func() {
				/* arrange */
				ss := newTestStateStore()
				stored := model.Auth{
					Creds:     model.Creds{Username: "user", Password: "pw"},
					Resources: "docker.io",
				}
				Expect(ss.applyAuthAdded(model.AuthAdded{Auth: stored})).To(Succeed())

				/* act */
				Expect(ss.applyAuthRemoved(model.AuthRemoved{Resources: "docker.io"})).To(Succeed())

				/* assert */
				Expect(ss.TryGetAuth("docker.io")).To(BeNil())
			})
		})
	})
	Context("ListAuths", func() {
		It("should return an empty slice when nothing is stored", func() {
			/* arrange */
			ss := newTestStateStore()

			/* act / assert */
			Expect(ss.ListAuths()).To(BeEmpty())
		})
		It("should return every stored auth entry", func() {
			/* arrange */
			ss := newTestStateStore()
			dockerAuth := model.Auth{
				Creds:     model.Creds{Username: "docker-user", Password: "docker-pw"},
				Resources: "docker.io",
			}
			githubAuth := model.Auth{
				Creds:     model.Creds{Username: "gh-user", Password: "gh-pw"},
				Resources: "github.com",
			}
			Expect(ss.applyAuthAdded(model.AuthAdded{Auth: dockerAuth})).To(Succeed())
			Expect(ss.applyAuthAdded(model.AuthAdded{Auth: githubAuth})).To(Succeed())

			/* act */
			actualAuths := ss.ListAuths()

			/* assert */
			Expect(actualAuths).To(ConsistOf(dockerAuth, githubAuth))
		})
		It("should omit entries removed via applyAuthRemoved", func() {
			/* arrange */
			ss := newTestStateStore()
			dockerAuth := model.Auth{
				Creds:     model.Creds{Username: "docker-user", Password: "docker-pw"},
				Resources: "docker.io",
			}
			githubAuth := model.Auth{
				Creds:     model.Creds{Username: "gh-user", Password: "gh-pw"},
				Resources: "github.com",
			}
			Expect(ss.applyAuthAdded(model.AuthAdded{Auth: dockerAuth})).To(Succeed())
			Expect(ss.applyAuthAdded(model.AuthAdded{Auth: githubAuth})).To(Succeed())

			/* act */
			Expect(ss.applyAuthRemoved(model.AuthRemoved{Resources: "docker.io"})).To(Succeed())

			/* assert */
			Expect(ss.ListAuths()).To(ConsistOf(githubAuth))
		})
		It("should match removes case-insensitively against the stored prefix", func() {
			/* arrange */
			ss := newTestStateStore()
			stored := model.Auth{
				Creds:     model.Creds{Username: "user", Password: "pw"},
				Resources: "docker.io",
			}
			Expect(ss.applyAuthAdded(model.AuthAdded{Auth: stored})).To(Succeed())

			/* act */
			Expect(ss.applyAuthRemoved(model.AuthRemoved{Resources: "DOCKER.IO"})).To(Succeed())

			/* assert */
			Expect(ss.ListAuths()).To(BeEmpty())
		})
	})
})
