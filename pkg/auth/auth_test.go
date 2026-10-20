package auth

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignedSessions(t *testing.T) {
	t.Setenv("AUTH_SECRET", strings.Repeat("s", 32))
	token, err := Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Parse(token)
	if err != nil || claims.Subject != "user-1" {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	if subject, err := Subject(r); err != nil || subject != "user-1" {
		t.Fatal(subject, err)
	}
	parts := strings.Split(token, ".")
	parts[0] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"another-user","exp":9999999999}`))
	if _, err := Parse(strings.Join(parts, ".")); err == nil {
		t.Fatal("accepted forged identity")
	}
	expired := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"sub":"user-1","exp":%d}`, time.Now().Add(-time.Minute).Unix())))
	if _, err := Parse(expired + "." + sign(expired)); err == nil {
		t.Fatal("accepted expired token")
	}
	t.Setenv("AUTH_SECRET", strings.Repeat("r", 32))
	if _, err := Parse(token); err == nil {
		t.Fatal("accepted token after key rotation")
	}
}

func TestRejectMissingSecret(t *testing.T) {
	t.Setenv("AUTH_SECRET", "")
	if _, err := Issue("user"); err == nil {
		t.Fatal("issued token without secret")
	}
}
