package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"proidentity/internal/devcrypto"
	"proidentity/internal/model"
)

// encryptedEnvelope is the wire format for encrypted request/response bodies.
type encryptedEnvelope struct {
	CT string `json:"ct"` // base64(nonce||ciphertext)
}

// deviceMiddleware decrypts incoming request bodies and encrypts outgoing responses
// for requests that carry an X-Device-ID header.
func (s *Server) deviceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deviceID := r.Header.Get("X-Device-ID")
		if deviceID == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Look up the installation and derive its shared key
		var inst model.Installation
		if err := s.db.Get(&inst, "SELECT * FROM installations WHERE id=?", deviceID); err != nil {
			jsonError(w, 401, "unknown device")
			return
		}

		if !inst.IsActive {
			jsonError(w, 401, "device revoked")
			return
		}

		aesKey, err := devcrypto.DeriveSharedKey(inst.ServerPrivateKey, inst.ClientPublicKey)
		if err != nil {
			jsonError(w, 500, "crypto error")
			return
		}

		aad := []byte(deviceID)

		// Decrypt request body (if present)
		if r.Body != nil && r.ContentLength != 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				jsonError(w, 400, "read body")
				return
			}
			r.Body.Close()

			if len(raw) > 0 {
				var env encryptedEnvelope
				if err := json.Unmarshal(raw, &env); err != nil || env.CT == "" {
					jsonError(w, 400, "invalid encrypted envelope")
					return
				}
				plain, err := devcrypto.Decrypt(aesKey, env.CT, aad)
				if err != nil {
					jsonError(w, 401, "decryption failed")
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(plain))
				r.ContentLength = int64(len(plain))
			}
		}

		// Update last_seen in background
		go s.db.Exec("UPDATE installations SET last_seen=? WHERE id=?", time.Now(), deviceID)

		// Wrap the ResponseWriter to capture and encrypt the response
		crw := &capturingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(crw, r)

		// Encrypt the captured body
		ct, err := devcrypto.Encrypt(aesKey, crw.body.Bytes(), aad)
		if err != nil {
			http.Error(w, `{"error":"encryption failed"}`, 500)
			return
		}

		env, _ := json.Marshal(encryptedEnvelope{CT: ct})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(crw.status)
		w.Write(env)
	})
}

// capturingResponseWriter captures the response body without writing it.
type capturingResponseWriter struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func (c *capturingResponseWriter) WriteHeader(status int) {
	c.status = status
	// Copy headers but don't call WriteHeader yet — we write after encryption
	for k, vs := range c.ResponseWriter.Header() {
		for _, v := range vs {
			c.ResponseWriter.Header().Set(k, v)
		}
	}
}

func (c *capturingResponseWriter) Write(b []byte) (int, error) {
	return c.body.Write(b)
}
