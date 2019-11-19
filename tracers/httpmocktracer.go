package tracers

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go/mocktracer"
	"net/http"
)

// HTTPMockTracer exposes mock tracer methods over HTTP
type HTTPMockTracer struct {
	tracer *mocktracer.MockTracer
}

type TestSpan struct {
	Span *mocktracer.MockSpan
	Tags map[string]interface{}
}

func (h *HTTPMockTracer) Reset(w http.ResponseWriter, r *http.Request) {
	h.tracer.Reset()
	w.Write([]byte("success"))
}

func (h *HTTPMockTracer) FinishedSpans(w http.ResponseWriter, r *http.Request) {
	spans := h.tracer.FinishedSpans()

	var finishedSpans []TestSpan

	for _, s := range spans {
		finishedSpans = append(finishedSpans, TestSpan{
			Span: s,
			Tags: s.Tags(),
		})
	}

	bs, err := json.Marshal(finishedSpans)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bs)
}

func Register(tracer *mocktracer.MockTracer, r *mux.Router) error {
	h := &HTTPMockTracer{
		tracer: tracer,
	}
	r.HandleFunc("/mocktracer/reset", h.Reset)
	r.HandleFunc("/mocktracer/finished-spans", h.FinishedSpans)
	return nil
}
