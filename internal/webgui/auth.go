package webgui

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"granger/internal/config"
)

const adminTokenPath = config.SecretsDir + "/admin-token"

func ensureAdminToken() error {
	if token, err := os.ReadFile(adminTokenPath); err == nil && len(strings.TrimSpace(string(token))) >= 32 {
		return nil
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(adminTokenPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(adminTokenPath, []byte(hex.EncodeToString(value)+"\n"), 0600)
}

func authorizedAdmin(r *http.Request, token string) bool {
	if validSession(r, token) {
		return true
	}
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") == false {
		return false
	}
	supplied := strings.TrimPrefix(header, "Bearer ")
	return len(token) >= 32 && subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) == 1
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := os.ReadFile(adminTokenPath)
		if r.URL.Path == "/session" {
			handleSession(w, r, strings.TrimSpace(string(token)))
			return
		}
		if authorizedAdmin(r, strings.TrimSpace(string(token))) == false {
			writeError(w, http.StatusForbidden, errors.New("administrator VPN connection or admin bearer token required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sessionValue(token string, expires int64) string {
	payload := strconv.FormatInt(expires, 10)
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func validSession(r *http.Request, token string) bool {
	cookie, err := r.Cookie("granger_session")
	if err != nil || len(token) < 32 {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expires <= time.Now().Unix() {
		return false
	}
	expected := sessionValue(token, expires)
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) == 1
}

func handleSession(w http.ResponseWriter, r *http.Request, token string) {
	if r.Header.Get("X-Granger-Request") != "1" {
		writeErrorText(w, http.StatusForbidden, "missing CSRF request header")
		return
	}
	if r.Method == http.MethodDelete {
		http.SetCookie(w, &http.Cookie{Name: "granger_session", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Secure: r.TLS != nil})
		writeJSON(w, http.StatusOK, apiResponse{OK: true})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(token) < 32 || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.Token)), []byte(token)) != 1 {
		writeErrorText(w, http.StatusForbidden, "invalid administrative token")
		return
	}
	expires := time.Now().Add(8 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "granger_session", Value: sessionValue(token, expires.Unix()), Path: "/", Expires: expires, MaxAge: 8 * 60 * 60, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	writeJSON(w, http.StatusOK, apiResponse{OK: true})
}
