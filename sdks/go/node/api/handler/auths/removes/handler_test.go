package removes

import (
	"bytes"
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
		Context("json.Decoder.Decode errors", func() {
			It("should return StatusCode of 400", func() {
				/* arrange */
				objectUnderTest := _handler{node: new(nodeFakes.FakeNode)}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodPost, urltemplates.Auths_Removes, bytes.NewReader([]byte("not-json")))
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusBadRequest))
			})
		})
		Context("node.RemoveAuth errors", func() {
			It("should return StatusCode 500 with the error message", func() {
				/* arrange */
				fakeNode := new(nodeFakes.FakeNode)
				fakeNode.RemoveAuthReturns(errors.New("expectedErr"))

				reqBody, _ := json.Marshal(model.RemoveAuthReq{Resources: "docker.io"})

				objectUnderTest := _handler{node: fakeNode}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodPost, urltemplates.Auths_Removes, bytes.NewReader(reqBody))
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
		Context("node.RemoveAuth succeeds", func() {
			It("should call node.RemoveAuth with the decoded req and return StatusNoContent", func() {
				/* arrange */
				fakeNode := new(nodeFakes.FakeNode)
				expectedReq := model.RemoveAuthReq{Resources: "docker.io"}
				reqBody, _ := json.Marshal(expectedReq)

				objectUnderTest := _handler{node: fakeNode}
				providedHTTPResp := httptest.NewRecorder()
				providedHTTPReq, err := http.NewRequest(http.MethodPost, urltemplates.Auths_Removes, bytes.NewReader(reqBody))
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusNoContent))
				Expect(fakeNode.RemoveAuthCallCount()).To(Equal(1))
				_, actualReq := fakeNode.RemoveAuthArgsForCall(0)
				Expect(actualReq).To(Equal(expectedReq))
			})
		})
	})
})
