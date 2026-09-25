package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		wantOK      bool
		wantStatus  int
	}{
		{"raw json", "application/json", `{"query":"red bra","budget":80}`, true, http.StatusOK},
		{"json with charset", "application/json; charset=utf-8", `{"query":"red bra"}`, true, http.StatusOK},
		{"form encoded", "application/x-www-form-urlencoded", `{"query":"red bra"}`, false, http.StatusUnsupportedMediaType},
		{"missing header", "", `{"query":"red bra"}`, false, http.StatusUnsupportedMediaType},
		{"malformed json", "application/json", `{"query":`, false, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/shop", strings.NewReader(tc.body))
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()

			var req ShopRequest
			ok := decodeJSON(w, r, &req)
			if ok != tc.wantOK || w.Code != tc.wantStatus {
				t.Fatalf("decodeJSON ok=%v status=%d, want ok=%v status=%d", ok, w.Code, tc.wantOK, tc.wantStatus)
			}
			if ok && req.Query != "red bra" {
				t.Fatalf("query = %q, want %q", req.Query, "red bra")
			}
		})
	}
}
