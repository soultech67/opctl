package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("Handler", func() {
	Context("NewHandler", func() {
		It("should not return nil", func() {
			Expect(NewHandler()).Should(Not(BeNil()))
		})
	})
	Context("Handle", func() {
		Context("next URL path segment is not empty", func() {
			It("returns 404", func() {
				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodGet, "blah", nil)
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusNotFound))
			})
		})
		Context("GET", func() {
			It("returns 200 with the current log state", func() {
				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodGet, "", nil)
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusOK))
				var state model.LogState
				Expect(json.Unmarshal(resp.Body.Bytes(), &state)).To(Succeed())
				Expect(model.LogLevels).To(ContainElement(state.Level))
			})
		})
		Context("POST with a valid level", func() {
			It("applies the level and returns it", func() {
				level := model.LogLevelDebug
				body, err := json.Marshal(model.SetLogStateReq{Level: &level})
				Expect(err).To(BeNil())

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusOK))
				var state model.LogState
				Expect(json.Unmarshal(resp.Body.Bytes(), &state)).To(Succeed())
				Expect(state.Level).To(Equal(model.LogLevelDebug))
			})
		})
		Context("POST toggling enablement", func() {
			It("returns the updated enablement", func() {
				enabled := false
				body, err := json.Marshal(model.SetLogStateReq{Enabled: &enabled})
				Expect(err).To(BeNil())

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusOK))
				var state model.LogState
				Expect(json.Unmarshal(resp.Body.Bytes(), &state)).To(Succeed())
				Expect(state.Enabled).To(BeFalse())
			})
		})
		Context("POST with an invalid level", func() {
			It("returns 400", func() {
				level := "bogus"
				body, err := json.Marshal(model.SetLogStateReq{Level: &level})
				Expect(err).To(BeNil())

				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusBadRequest))
			})
		})
		Context("unsupported method", func() {
			It("returns 405", func() {
				resp := httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodDelete, "", nil)
				Expect(err).To(BeNil())

				NewHandler().Handle(resp, req)

				Expect(resp.Code).To(Equal(http.StatusMethodNotAllowed))
			})
		})
	})
})
