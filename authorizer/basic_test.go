//
// Copyright (C) 2021 - 2025 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the Apache License Version 2.0. See the LICENSE file for details.
// https://github.com/fogfish/swarm
//

package authorizer_test

import (
	"testing"

	"github.com/fogfish/it/v2"
	"github.com/fogfish/scud/authorizer"
)

func TestBasic(t *testing.T) {
	auth, err := authorizer.NewBasic("access", "secret")
	it.Then(t).Must(it.Nil(err))

	t.Run("Success", func(t *testing.T) {
		access, token, err := auth.Validate("YWNjZXNzOnNlY3JldA")
		it.Then(t).Should(
			it.Nil(err),
			it.Equal(access, "access"),
			it.Map(token).Have("auth", "basic"),
			it.Map(token).Have("sub", "access"),
		)
	})

	t.Run("Forbidden/InvalidKey", func(t *testing.T) {
		_, _, err := auth.Validate("YWNjZXNzOnNlY3JldHo")
		it.Then(t).ShouldNot(
			it.Nil(err),
		)
	})

	t.Run("Forbidden/Format", func(t *testing.T) {
		_, _, err := auth.Validate("YWNjZXNzc2VjcmV0")
		it.Then(t).ShouldNot(
			it.Nil(err),
		)
	})

	t.Run("Forbidden/Corrupted", func(t *testing.T) {
		_, _, err := auth.Validate(".")
		it.Then(t).ShouldNot(
			it.Nil(err),
		)
	})

}
