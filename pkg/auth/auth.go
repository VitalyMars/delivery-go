// Package auth implements short-lived signed sessions for the local application.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")

type Claims struct {
	Subject   string `json:"sub"`
	ExpiresAt int64  `json:"exp"`
}

func ValidateConfig() error {
	if len(os.Getenv("AUTH_SECRET")) < 32 {
		return errors.New("AUTH_SECRET must contain at least 32 bytes")
	}
	return nil
}

func Issue(subject string) (string, error) {
	if err := ValidateConfig(); err != nil {
		return "", err
	}
	if subject == "" {
		return "", ErrUnauthorized
	}
	payload, err := json.Marshal(Claims{subject, time.Now().Add(time.Hour).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + sign(encoded), nil
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(os.Getenv("AUTH_SECRET")))
	mac.Write([]byte("delivery-session-v1:" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func Parse(token string) (Claims, error) {
	var claims Claims
	if ValidateConfig() != nil || len(token) > 2048 {
		return claims, ErrUnauthorized
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(sign(parts[0])), []byte(parts[1])) {
		return claims, ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(payload, &claims) != nil || claims.Subject == "" || claims.ExpiresAt <= time.Now().Unix() {
		return Claims{}, ErrUnauthorized
	}
	return claims, nil
}

func Subject(r *http.Request) (string, error) {
	header := strings.Fields(r.Header.Get("Authorization"))
	if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
		return "", ErrUnauthorized
	}
	claims, err := Parse(header[1])
	return claims.Subject, err
}
