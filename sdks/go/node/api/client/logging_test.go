package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang-interfaces/ihttp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/urltemplates"
)

var _ = Context("LogControl", func() {
	expectedState := model.LogState{
		Enabled:  true,
		Level:    model.LogLevelDebug,
		Filepath: "/data/logs/node.log",
		Format:   "text",
	}
	stateJSON, _ := json.Marshal(expectedState)

	okResponse := func() *http.Response {
		return &http.Response{
			Body:       io.NopCloser(strings.NewReader(string(stateJSON))),
			StatusCode: http.StatusOK,
		}
	}

	Context("NewLogControlClient", func() {
		It("should not return nil", func() {
			Expect(NewLogControlClient(url.URL{})).ShouldNot(BeNil())
		})
	})

	Context("GetLogState", func() {
		It("GETs /logging and returns the decoded state", func() {
			/* arrange */
			fakeHTTPClient := new(ihttp.FakeClient)
			fakeHTTPClient.DoReturns(okResponse(), nil)
			objectUnderTest := apiClient{httpClient: fakeHTTPClient}

			/* act */
			actualState, actualErr := objectUnderTest.GetLogState(context.TODO())

			/* assert */
			Expect(actualErr).To(BeNil())
			Expect(actualState).To(Equal(expectedState))

			actualReq := fakeHTTPClient.DoArgsForCall(0)
			Expect(actualReq.Method).To(Equal("GET"))
			Expect(actualReq.URL.Path).To(Equal(urltemplates.Logging))
			Expect(actualReq.Body).To(BeNil())
		})
	})

	Context("SetLogState", func() {
		It("POSTs the request body to /logging and returns the decoded state", func() {
			/* arrange */
			fakeHTTPClient := new(ihttp.FakeClient)
			fakeHTTPClient.DoReturns(okResponse(), nil)
			objectUnderTest := apiClient{httpClient: fakeHTTPClient}

			level := model.LogLevelWarn
			enabled := false

			/* act */
			actualState, actualErr := objectUnderTest.SetLogState(
				context.TODO(),
				model.SetLogStateReq{Level: &level, Enabled: &enabled},
			)

			/* assert */
			Expect(actualErr).To(BeNil())
			Expect(actualState).To(Equal(expectedState))

			actualReq := fakeHTTPClient.DoArgsForCall(0)
			Expect(actualReq.Method).To(Equal("POST"))
			Expect(actualReq.URL.Path).To(Equal(urltemplates.Logging))

			bodyBytes, _ := io.ReadAll(actualReq.Body)
			var sent model.SetLogStateReq
			Expect(json.Unmarshal(bodyBytes, &sent)).To(Succeed())
			Expect(sent.Level).ToNot(BeNil())
			Expect(*sent.Level).To(Equal(model.LogLevelWarn))
			Expect(sent.Enabled).ToNot(BeNil())
			Expect(*sent.Enabled).To(BeFalse())
		})
	})

	Context("StatusCode != 200", func() {
		It("returns the response body as an error", func() {
			/* arrange */
			fakeHTTPClient := new(ihttp.FakeClient)
			fakeHTTPClient.DoReturns(
				&http.Response{
					Body:       io.NopCloser(strings.NewReader("boom")),
					StatusCode: http.StatusInternalServerError,
				},
				nil,
			)
			objectUnderTest := apiClient{httpClient: fakeHTTPClient}

			/* act */
			_, actualErr := objectUnderTest.GetLogState(context.TODO())

			/* assert */
			Expect(actualErr).To(MatchError("boom"))
		})
	})
})
