package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousAndOversizedBodies(t *testing.T) {
	for _, body := range []string{`{"name":"one"} {"name":"two"}`, `{"unknown":1}`, `{"name":"` + strings.Repeat("x", 1<<20) + `"}`} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		var target struct {
			Name string `json:"name"`
		}
		if err := Decode(httptest.NewRecorder(), request, &target); err == nil {
			t.Fatal("accepted invalid body")
		}
	}
}

func TestAuthRateLimit(t *testing.T) {
	handler := LimitAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for i := 0; i < 11; i++ {
		r := httptest.NewRequest("POST", "/users/login", nil)
		r.RemoteAddr = "127.0.0.1:42"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if i < 10 && w.Code != 204 || i == 10 && w.Code != 429 {
			t.Fatal(i, w.Code)
		}
	}
}

func TestBrokersSplitAndTrim(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", " a:9092, b:9092 , ")
	got := Brokers()
	if len(got) != 2 || got[0] != "a:9092" || got[1] != "b:9092" {
		t.Fatal(got)
	}
}
