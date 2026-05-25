package lists

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/urltemplates"
	nodeFakes "github.com/opctl/opctl/sdks/go/node/fakes"
)

var _ = Context("Handler", func() {
	Context("NewHandler", func() {
		It("should not return nil", func() {
			/* arrange/act/assert */
			Expect(NewHandler(new(nodeFakes.FakeNode))).Should(Not(BeNil()))
		})
	})
	Context("Handle", func() {
		Context("node.ListAuths errors", func() {
			It("should return StatusCode of 500 with the error message", func() {
				/* arrange */
				fakeNode := new(nodeFakes.FakeNode)
				fakeNode.ListAuthsReturns(nil, errors.New("expectedErr"))

				objectUnderTest := _handler{node: fakeNode}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodGet, urltemplates.Auths_Lists, nil)
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusInternalServerError))
				body, _ := io.ReadAll(providedHTTPResp.Body)
				Expect(string(body)).To(ContainSubstring("expectedErr"))
			})
		})
		Context("node.ListAuths returns auths", func() {
			It("should return StatusCode 200 with the auths encoded as JSON", func() {
				/* arrange */
				expectedAuths := []model.Auth{
					{Resources: "docker.io", Creds: model.Creds{Username: "user1", Password: "pw1"}},
					{Resources: "github.com", Creds: model.Creds{Username: "user2", Password: "pw2"}},
				}
				fakeNode := new(nodeFakes.FakeNode)
				fakeNode.ListAuthsReturns(expectedAuths, nil)

				objectUnderTest := _handler{node: fakeNode}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodGet, urltemplates.Auths_Lists, nil)
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusOK))
				Expect(providedHTTPResp.Header().Get("Content-Type")).To(Equal("application/json"))

				var actualAuths []model.Auth
				Expect(json.NewDecoder(providedHTTPResp.Body).Decode(&actualAuths)).To(Succeed())
				Expect(actualAuths).To(Equal(expectedAuths))
			})
		})
		Context("node.ListAuths returns nil slice", func() {
			It("should return StatusCode 200 with an empty JSON array", func() {
				/* arrange */
				fakeNode := new(nodeFakes.FakeNode)
				fakeNode.ListAuthsReturns(nil, nil)

				objectUnderTest := _handler{node: fakeNode}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodGet, urltemplates.Auths_Lists, nil)
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusOK))
				var actualAuths []model.Auth
				Expect(json.NewDecoder(providedHTTPResp.Body).Decode(&actualAuths)).To(Succeed())
				Expect(actualAuths).To(BeEmpty())
			})
		})
	})
})
