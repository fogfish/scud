//
// Copyright (C) 2020 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//

package authorizer_test

import (
	"encoding/json"
	"testing"

	"github.com/fogfish/it/v2"
	"github.com/fogfish/scud/authorizer"
)

func request(headers map[string]string) authorizer.Request {
	return authorizer.Request{Headers: headers}
}

func TestAllowDeny(t *testing.T) {
	allow, err := json.Marshal(authorizer.Allow(map[string]any{"sub": "x"}))
	it.Then(t).Must(it.Nil(err))

	deny, err := json.Marshal(authorizer.Deny())
	it.Then(t).Must(it.Nil(err))

	it.Then(t).Should(
		it.Equal(string(allow), `{"isAuthorized":true,"context":{"sub":"x"}}`),
		it.Equal(string(deny), `{"isAuthorized":false}`),
	)
}

func TestHeader(t *testing.T) {
	req := request(map[string]string{"x-api-key": "a", "X-Other": "b"})

	key, ok := authorizer.Header(req, "X-Api-Key")
	it.Then(t).Should(it.True(ok), it.Equal(key, "a"))

	other, ok := authorizer.Header(req, "x-other")
	it.Then(t).Should(it.True(ok), it.Equal(other, "b"))

	_, ok = authorizer.Header(req, "authorization")
	it.Then(t).ShouldNot(it.True(ok))
}

func TestQuery(t *testing.T) {
	req := authorizer.Request{QueryStringParameters: map[string]string{"apikey": "k"}}

	key, ok := authorizer.Query(req, "apikey")
	it.Then(t).Should(it.True(ok), it.Equal(key, "k"))

	_, ok = authorizer.Query(req, "other")
	it.Then(t).ShouldNot(it.True(ok))
}

func TestBearer(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		token, ok := authorizer.Bearer(request(map[string]string{"authorization": "Bearer abc"}))
		it.Then(t).Should(it.True(ok), it.Equal(token, "abc"))
	})

	t.Run("CaseInsensitiveScheme", func(t *testing.T) {
		token, ok := authorizer.Bearer(request(map[string]string{"authorization": "bearer abc"}))
		it.Then(t).Should(it.True(ok), it.Equal(token, "abc"))
	})

	for name, header := range map[string]map[string]string{
		"Anonymous":   {},
		"OtherScheme": {"authorization": "Basic abc"},
		"NoToken":     {"authorization": "Bearer"},
		"EmptyToken":  {"authorization": "Bearer  "},
	} {
		t.Run("Refused/"+name, func(t *testing.T) {
			_, ok := authorizer.Bearer(request(header))
			it.Then(t).ShouldNot(it.True(ok))
		})
	}
}

func TestBasicCredentials(t *testing.T) {
	for name, header := range map[string]string{
		"Padded":   "Basic YWNjZXNzOnNlY3JldA==",
		"Unpadded": "Basic YWNjZXNzOnNlY3JldA",
	} {
		t.Run(name, func(t *testing.T) {
			user, pass, ok := authorizer.BasicCredentials(request(map[string]string{"authorization": header}))
			it.Then(t).Should(
				it.True(ok),
				it.Equal(user, "access"),
				it.Equal(pass, "secret"),
			)
		})
	}

	for name, header := range map[string]map[string]string{
		"Anonymous":   {},
		"OtherScheme": {"authorization": "Bearer YWNjZXNzOnNlY3JldA=="},
		"Corrupted":   {"authorization": "Basic ."},
		"NoSeparator": {"authorization": "Basic YWNjZXNzc2VjcmV0"},
	} {
		t.Run("Refused/"+name, func(t *testing.T) {
			_, _, ok := authorizer.BasicCredentials(request(header))
			it.Then(t).ShouldNot(it.True(ok))
		})
	}
}
