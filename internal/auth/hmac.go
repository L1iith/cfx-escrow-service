package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Verifier struct {
	secret       []byte
	window       time.Duration
	maxBodyBytes int64
	now          func() time.Time
}

func NewVerifier(secret string, window time.Duration, maxBodyBytes int64) *Verifier {
	return &Verifier{
		secret:       []byte(secret),
		window:       window,
		maxBodyBytes: maxBodyBytes,
		now:          time.Now,
	}
}

func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte{'\n'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp := strings.TrimSpace(r.Header.Get("X-Escrow-Timestamp"))
		signature := strings.TrimSpace(r.Header.Get("X-Escrow-Signature"))
		seconds, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || signature == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		at := time.Unix(seconds, 0)
		delta := v.now().Sub(at)
		if delta < 0 {
			delta = -delta
		}
		if delta > v.window {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, v.maxBodyBytes+1))
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if int64(len(body)) > v.maxBodyBytes {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		expected, err := hex.DecodeString(Sign(string(v.secret), timestamp, body))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided, err := hex.DecodeString(signature)
		if err != nil || !hmac.Equal(expected, provided) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
