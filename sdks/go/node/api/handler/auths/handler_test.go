package auths

import (
	"net/http"
	"net/http/httptest"
	"strings"

	addsFakes "github.com/opctl/opctl/sdks/go/node/api/handler/auths/adds/fakes"
	listsFakes "github.com/opctl/opctl/sdks/go/node/api/handler/auths/lists/fakes"
	removesFakes "github.com/opctl/opctl/sdks/go/node/api/handler/auths/removes/fakes"
	nodeFakes "github.com/opctl/opctl/sdks/go/node/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("Handler", func() {
	Context("NewHandler", func() {
		It("should not return nil", func() {
			/* arrange/act/assert */
			Expect(NewHandler(new(nodeFakes.FakeNode))).Should(Not(BeNil()))
		})
	})
	Context("Handle", func() {
		Context("next URL path segment isn't a known auths subresource", func() {
			It("should return StatusNotFound", func() {
				/* arrange */
				objectUnderTest := _handler{}
				providedHTTPResp := httptest.NewRecorder()

				providedHTTPReq, err := http.NewRequest("dummyMethod", "", nil)
				if err != nil {
					panic(err.Error())
				}

				/* act */
				objectUnderTest.Handle(providedHTTPResp, providedHTTPReq)

				/* assert */
				Expect(providedHTTPResp.Code).To(Equal(http.StatusNotFound))
			})
		})
		Context("next URL path segment is adds", func() {
			It("should call addsHandler.Handle w/ expected args", func() {
				/* arrange */
				fakeAddsHandler := new(addsFakes.FakeHandler)

				objectUnderTest := _handler{
					addsHandler: fakeAddsHandler,
				}

				providedPath := "adds/dummy"
				providedHTTPReq, err := http.NewRequest("dummyMethod", providedPath, nil)
				if err != nil {
					panic(err.Error())
				}

				expectedURLPath := strings.SplitN(providedPath, "/", 2)[1]

				/* act */
				objectUnderTest.Handle(httptest.NewRecorder(), providedHTTPReq)

				/* assert */
				_, actualHTTPReq := fakeAddsHandler.HandleArgsForCall(0)

				Expect(actualHTTPReq.URL.Path).To(Equal(expectedURLPath))

				// this works because our URL path set mutates the httpRequest
				Expect(actualHTTPReq).To(Equal(providedHTTPReq))
			})
		})
		Context("next URL path segment is lists", func() {
			It("should call listsHandler.Handle w/ expected args", func() {
				/* arrange */
				fakeListsHandler := new(listsFakes.FakeHandler)

				objectUnderTest := _handler{
					listsHandler: fakeListsHandler,
				}

				providedPath := "lists/dummy"
				providedHTTPReq, err := http.NewRequest("dummyMethod", providedPath, nil)
				if err != nil {
					panic(err.Error())
				}

				expectedURLPath := strings.SplitN(providedPath, "/", 2)[1]

				/* act */
				objectUnderTest.Handle(httptest.NewRecorder(), providedHTTPReq)

				/* assert */
				_, actualHTTPReq := fakeListsHandler.HandleArgsForCall(0)

				Expect(actualHTTPReq.URL.Path).To(Equal(expectedURLPath))
				Expect(actualHTTPReq).To(Equal(providedHTTPReq))
			})
		})
		Context("next URL path segment is removes", func() {
			It("should call removesHandler.Handle w/ expected args", func() {
				/* arrange */
				fakeRemovesHandler := new(removesFakes.FakeHandler)

				objectUnderTest := _handler{
					removesHandler: fakeRemovesHandler,
				}

				providedPath := "removes/dummy"
				providedHTTPReq, err := http.NewRequest("dummyMethod", providedPath, nil)
				if err != nil {
					panic(err.Error())
				}

				expectedURLPath := strings.SplitN(providedPath, "/", 2)[1]

				/* act */
				objectUnderTest.Handle(httptest.NewRecorder(), providedHTTPReq)

				/* assert */
				_, actualHTTPReq := fakeRemovesHandler.HandleArgsForCall(0)

				Expect(actualHTTPReq.URL.Path).To(Equal(expectedURLPath))
				Expect(actualHTTPReq).To(Equal(providedHTTPReq))
			})
		})
	})
})
