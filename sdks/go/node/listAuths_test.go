package node

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("core", func() {
	Context("ListAuths", func() {
		It("should return entries previously applied to the state store", func() {
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

			objectUnderTest := core{stateStore: ss}

			/* act */
			auths, err := objectUnderTest.ListAuths(context.Background())

			/* assert */
			Expect(err).ToNot(HaveOccurred())
			Expect(auths).To(ConsistOf(dockerAuth, githubAuth))
		})
		It("should return an empty slice when nothing is stored", func() {
			/* arrange */
			ss := newTestStateStore()
			objectUnderTest := core{stateStore: ss}

			/* act */
			auths, err := objectUnderTest.ListAuths(context.Background())

			/* assert */
			Expect(err).ToNot(HaveOccurred())
			Expect(auths).To(BeEmpty())
		})
	})
})
