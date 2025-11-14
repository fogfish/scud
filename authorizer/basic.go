//
// Copyright (C) 2021 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the Apache License Version 2.0. See the LICENSE file for details.
// https://github.com/fogfish/swarm
//

package authorizer

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
)

var (
	ErrForbidden = errors.New("forbidden")
)

// Basic Authorizer implements simple access/secret key validation.
type Basic struct {
	access, secret string
}

func NewBasic(access, secret string) (*Basic, error) {
	if access == "" || secret == "" {
		return nil, errors.New("basic auth is not configured")
	}

	return &Basic{
		access: access,
		secret: secret,
	}, nil
}

func (auth *Basic) Validate(apikey string) (string, map[string]any, error) {
	c, err := base64.RawStdEncoding.DecodeString(apikey)
	if err != nil {
		slog.Error("corrupted apikey.")
		return "", nil, ErrForbidden
	}

	access, secret, ok := strings.Cut(string(c), ":")
	if !ok {
		slog.Error("malformed apikey.")
		return "", nil, ErrForbidden
	}

	gaccess := sha256.Sum256([]byte(access))
	gsecret := sha256.Sum256([]byte(secret))
	haccess := sha256.Sum256([]byte(auth.access))
	hsecret := sha256.Sum256([]byte(auth.secret))

	accessMatch := (subtle.ConstantTimeCompare(gaccess[:], haccess[:]) == 1)
	secretMatch := (subtle.ConstantTimeCompare(gsecret[:], hsecret[:]) == 1)

	if !(accessMatch && secretMatch) {
		slog.Error("apikey forbidden.")
		return "", nil, ErrForbidden
	}

	return access, map[string]any{"auth": "basic", "sub": access}, nil
}
