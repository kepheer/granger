package webgui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSNXCancelDoesNotWaitForRoutingMutation(t *testing.T) {
	mutationMu.Lock()
	defer mutationMu.Unlock()

	handler := requireMutationHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodPost, "/snx/corporate/cancel", nil)
	request.Header.Set("X-Granger-Request", "1")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case <-done:
		if response.Code != http.StatusOK {
			t.Fatalf("cancel status = %d", response.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel blocked behind a routing mutation")
	}
}

func TestAsyncSNXStillRequiresCSRFHeader(t *testing.T) {
	handler := requireMutationHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unauthorized mutation reached handler")
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/snx/corporate/start", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF header status = %d", response.Code)
	}
}
