package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid,omitempty"`
}

type jwtPayload interface {
	decode(s string) error
}

type customToken struct {
	Iss    string                 `json:"iss"`
	Aud    string                 `json:"aud"`
	Exp    int64                  `json:"exp"`
	Iat    int64                  `json:"iat"`
	Sub    string                 `json:"sub,omitempty"`
	UID    string                 `json:"uid,omitempty"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

func (p *customToken) decode(s string) error {
	return decode(s, p)
}

func (t *Token) decode(s string) error {
	claims := make(map[string]interface{})
	if err := decode(s, &claims); err != nil {
		return err
	}
	if err := decode(s, t); err != nil {
		return err
	}

	for _, r := range []string{"iss", "aud", "exp", "iat", "sub", "uid"} {
		delete(claims, r)
	}
	t.Claims = claims
	return nil
}

func defaultHeader() jwtHeader {
	return jwtHeader{Algorithm: "RS256", Type: "JWT"}
}

func encode(i interface{}) (string, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decode(s string, i interface{}) error {
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(bytes.NewBuffer(decoded)).Decode(i); err != nil {
		return err
	}
	return nil
}

func encodeToken(h jwtHeader, p jwtPayload, s Signer) (string, error) {
	header, err := encode(h)
	if err != nil {
		return "", err
	}
	payload, err := encode(p)
	if err != nil {
		return "", err
	}

	ss := fmt.Sprintf("%s.%s", header, payload)
	sig, err := s.Sign(ss)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", ss, base64.RawURLEncoding.EncodeToString(sig)), nil
}

func decodeToken(token string, ks keySource, h *jwtHeader, p jwtPayload) error {
	s := strings.Split(token, ".")
	if len(s) != 3 {
		return errors.New("incorrect number of segments")
	}

	if err := decode(s[0], h); err != nil {
		return err
	}
	if err := p.decode(s[1]); err != nil {
		return err
	}

	keys, err := ks.Keys()
	if err != nil {
		return err
	}
	verified := false
	for _, k := range keys {
		if h.KeyID == "" || h.KeyID == k.Kid {
			if verifySignature(s, k) == nil {
				verified = true
				break
			}
		}
	}

	if !verified {
		return errors.New("failed to verify token signature")
	}
	return nil
}
