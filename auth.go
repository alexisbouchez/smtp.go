package smtp

import (
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// SASLMechanism defines a client-side SASL authentication mechanism.
type SASLMechanism interface {
	// Name returns the IANA-registered mechanism name (e.g., "PLAIN").
	Name() string
	// Start begins authentication and returns the initial response.
	// If no initial response is needed, return nil, nil.
	Start() ([]byte, error)
	// Next processes a server challenge and returns the response.
	Next(challenge []byte) ([]byte, error)
}

// PlainAuth returns a SASLMechanism implementing SASL PLAIN (RFC 4616).
// The identity is typically empty (server derives it from username).
func PlainAuth(identity, username, password string) SASLMechanism {
	return &plainAuth{identity: identity, username: username, password: password}
}

type plainAuth struct {
	identity string
	username string
	password string
}

func (a *plainAuth) Name() string { return "PLAIN" }

func (a *plainAuth) Start() ([]byte, error) {
	// PLAIN format: [authzid] NUL authcid NUL passwd
	resp := []byte(a.identity + "\x00" + a.username + "\x00" + a.password)
	return resp, nil
}

func (a *plainAuth) Next(challenge []byte) ([]byte, error) {
	return nil, fmt.Errorf("smtp: unexpected PLAIN challenge")
}

// LoginAuth returns a SASLMechanism implementing the LOGIN mechanism
// (draft-murchison-sasl-login, widely deployed).
type loginAuth struct {
	username string
	password string
	step     int
}

// LoginAuth returns a SASLMechanism implementing SASL LOGIN.
func LoginAuth(username, password string) SASLMechanism {
	return &loginAuth{username: username, password: password}
}

func (a *loginAuth) Name() string { return "LOGIN" }

func (a *loginAuth) Start() ([]byte, error) {
	// LOGIN does not have an initial response; the server sends challenges.
	return nil, nil
}

func (a *loginAuth) Next(challenge []byte) ([]byte, error) {
	switch a.step {
	case 0:
		a.step++
		return []byte(a.username), nil
	case 1:
		a.step++
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp: unexpected LOGIN challenge at step %d", a.step)
	}
}

// CramMD5Auth returns a SASLMechanism implementing SASL CRAM-MD5 (RFC 2195).
func CramMD5Auth(username, secret string) SASLMechanism {
	return &cramMD5Auth{username: username, secret: secret}
}

type cramMD5Auth struct {
	username string
	secret   string
}

func (a *cramMD5Auth) Name() string { return "CRAM-MD5" }

func (a *cramMD5Auth) Start() ([]byte, error) {
	// CRAM-MD5 does not have an initial response; server sends the challenge.
	return nil, nil
}

func (a *cramMD5Auth) Next(challenge []byte) ([]byte, error) {
	// HMAC-MD5 of challenge using secret as key.
	mac := hmac.New(md5.New, []byte(a.secret))
	mac.Write(challenge)
	digest := hex.EncodeToString(mac.Sum(nil))
	return []byte(a.username + " " + digest), nil
}
