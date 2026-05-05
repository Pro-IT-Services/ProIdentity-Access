package api

import (
	"encoding/json"
	"log"
	"net/http"
)

const maxRequestBodyBytes = 1 << 20

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	if code >= http.StatusInternalServerError {
		log.Printf("api error %d: %s", code, msg)
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
