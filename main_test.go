package main_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	garaj "github.com/talentlessguy/garaj"
)

func TestGenerateSecureToken(t *testing.T) {
	token, err := garaj.GenerateSecureToken(32)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 32 {
		t.Errorf("token length is %d, expected 32", len(token))
	}
}

func TestPutHandler(t *testing.T) {
	token, err := garaj.GenerateSecureToken(32)
	if err != nil {
		t.Fatal(err)
	}
	maxBodyBytes := int64(64) << 20
	handler := garaj.PutHandler(token, maxBodyBytes)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/put", nil)
	handler(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusMethodNotAllowed)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodOptions, "/put", nil)
	handler(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusNoContent)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", nil)
	r.Header.Set("X-API-Key", "invalid-token")
	handler(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusForbidden)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", nil)
	r.Header.Set("X-API-Key", token)
	handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusBadRequest)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", nil)
	r.Header.Set("X-API-Key", token)
	r.Header.Set("Content-Type", "invalid-content-type")
	handler(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusUnsupportedMediaType)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", nil)
	r.Header.Set("X-API-Key", token)
	r.Header.Set("Content-Type", garaj.CarContentType)
	handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusOK)
	}

	// Testing with huge binary file
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", bytes.NewReader(make([]byte, int64(64)<<20)))
	r.Header.Set("X-API-Key", token)
	r.Header.Set("Content-Type", garaj.CarContentType)
	handler(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusRequestEntityTooLarge)
	}

	// Testing with an actual CAR file
	carFile, err := os.ReadFile("./fixtures/test.car")
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/put", bytes.NewReader(carFile))

	r.Header.Set("X-API-Key", token)
	r.Header.Set("Content-Type", garaj.CarContentType)
	handler(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusInternalServerError)
	}
}
