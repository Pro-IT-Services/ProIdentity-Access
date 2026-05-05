package admin

import (
	"encoding/json"
	"log"
	"net/http"

	"proidentity/internal/auth"
)

const maxRequestBodyBytes = 1 << 20

func claimsFrom(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(auth.ClaimsCtxKey).(*auth.Claims)
	return c
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	if code >= http.StatusInternalServerError {
		log.Printf("admin api error %d: %s", code, msg)
		msg = "internal server error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodyBytes)
	return json.NewDecoder(r.Body).Decode(v)
}
