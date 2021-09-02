// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/context"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid,omitempty"`
}

type jwtPayload interface {
	decodeFrom(s string) error
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

func (p *customToken) decodeFrom(s string) error {
	return decode(s, p)
}

// decode accepts a JWT segment, and decodes it into the given interface.
func decode(s string, i interface{}) error {
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewBuffer(decoded)).Decode(i)
}

func encodeToken(ctx context.Context, s signer, h jwtHeader, p jwtPayload) (string, error) {
	encode := func(i interface{}) (string, error) {
		b, err := json.Marshal(i)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	header, err := encode(h)
	if err != nil {
		return "", err
	}
	payload, err := encode(p)
	if err != nil {
		return "", err
	}

	ss := fmt.Sprintf("%s.%s", header, payload)
	sig, err := s.Sign(ctx, []byte(ss))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", ss, base64.RawURLEncoding.EncodeToString(sig)), nil
}

func decodeToken(ctx context.Context, token string, ks keySource, h *jwtHeader, p jwtPayload) error {
	s := strings.Split(token, ".")
	if len(s) != 3 {
		return errors.New("incorrect number of segments")
	}

	if err := decode(s[0], h); err != nil {
		return err
	}
	if err := p.decodeFrom(s[1]); err != nil {
		return err
	}

	keys, err := ks.Keys(ctx)
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
