package webgui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminSessionIsHttpOnlyExpiresAndRejectsForgery(t *testing.T) {
	token := strings.Repeat("a", 64)
	req := httptest.NewRequest("POST", "/session", strings.NewReader(`{"token":"`+token+`"}`))
	req.Header.Set("X-Granger-Request", "1")
	response := httptest.NewRecorder()
	handleSession(response, req, token)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].HttpOnly == false || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe admin session cookie")
	}
	next := httptest.NewRequest("GET", "/config", nil)
	next.AddCookie(cookies[0])
	if validSession(next, token) == false {
		t.Fatal("valid session rejected")
	}
	if validSession(next, strings.Repeat("b", 64)) {
		t.Fatal("session accepted with wrong signing key")
	}
	expired := httptest.NewRequest("GET", "/config", nil)
	expired.AddCookie(&http.Cookie{Name: "granger_session", Value: sessionValue(token, time.Now().Add(-time.Minute).Unix())})
	if validSession(expired, token) {
		t.Fatal("expired session accepted")
	}
}
