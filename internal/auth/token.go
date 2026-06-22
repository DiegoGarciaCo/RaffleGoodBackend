package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// SessionCookieName is better-auth's default session cookie. If you enable
// secure cookies in better-auth, it becomes "__Secure-better-auth.session_token".
const SessionCookieName = "better-auth.session_token"

// ExtractToken pulls the session token from either the Authorization bearer
// header (mobile clients) or the better-auth session cookie (web clients).
//
// better-auth signs the value as "<token>.<signature>" but stores only <token>
// in the session table, so we strip the signature and return the raw token.
func ExtractToken(r *http.Request) string {
	var raw string

	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			raw = strings.TrimSpace(after)
		}
	}
	if raw == "" {
		if c, err := r.Cookie(SessionCookieName); err == nil {
			raw = c.Value
		}
	}
	if raw == "" {
		return ""
	}

	// "<token>.<signature>" → "<token>". Tokens and base64url signatures
	// don't contain '.', so the first dot is the separator.
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		return raw[:i]
	}
	return raw
}

// VerifySignature is optional defense-in-depth. It checks better-auth's HMAC
// signature on a raw "<token>.<signature>" value using your BETTER_AUTH_SECRET.
//
// The session-table lookup is the primary check (the token is a secret), so
// this is opt-in. Enable it in the auth middleware only if you've confirmed
// your better-auth version signs tokens this way; signing details vary by
// version, and a mismatch here would reject all requests.
func VerifySignature(signedValue, secret string) bool {
	i := strings.IndexByte(signedValue, '.')
	if i < 0 {
		return false
	}
	token, sig := signedValue[:i], signedValue[i+1:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(token))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1
}
