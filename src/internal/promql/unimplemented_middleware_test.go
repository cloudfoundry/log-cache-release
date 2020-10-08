package promql_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/log-cache/internal/promql"

	"github.com/Benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UnimplementedMiddleware", func() {
	It("return 200 for /api/v1/anything", func() {
		baseHandlerCalled := false
		spyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			baseHandlerCalled = true
		})

		promMiddleware := promql.UnimplementedMiddleware(spyHandler)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
		promMiddleware.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(baseHandlerCalled).To(BeTrue())
	})
	It("return 501 for /api/v1/query", func() {
		baseHandlerCalled := false
		spyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			baseHandlerCalled = true
		})

		promMiddleware := promql.UnimplementedMiddleware(spyHandler)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
		promMiddleware.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusNotImplemented))
		Expect(recorder.Body.String()).To(unmarshalledmatchers.ContainUnorderedJSON(`{
				"status": "error",
				"errorType": "bad_data",
				"error": "Metrics not available"
			}`))
		Expect(baseHandlerCalled).To(BeFalse())
	})
	It("return 501 for /api/v1/query_range", func() {
		baseHandlerCalled := false
		spyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			baseHandlerCalled = true
		})

		promMiddleware := promql.UnimplementedMiddleware(spyHandler)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/query_range", nil)
		promMiddleware.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusNotImplemented))
		Expect(recorder.Body.String()).To(unmarshalledmatchers.ContainUnorderedJSON(`{
				"status": "error",
				"errorType": "bad_data",
				"error": "Metrics not available"
			}`))
		Expect(baseHandlerCalled).To(BeFalse())
	})
})
