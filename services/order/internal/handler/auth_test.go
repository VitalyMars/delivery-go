package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VitalyMars/delivery_go/pkg/auth"
	"github.com/google/uuid"
)

func TestCreateOrderRejectsForgedUserBeforeCallingService(t *testing.T) {
	t.Setenv("AUTH_SECRET", strings.Repeat("s", 32))
	handler := NewOrderHandler(nil)
	for _, authenticated := range []bool{false, true} {
		request := httptest.NewRequest("POST", "/orders", strings.NewReader(`{"user_id":"`+uuid.NewString()+`"}`))
		if authenticated {
			token, err := auth.Issue(uuid.NewString())
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.CreateOrder(response, request)
		expected := 401
		if authenticated {
			expected = 403
		}
		if response.Code != expected {
			t.Fatal(response.Code, response.Body.String())
		}
	}
}
