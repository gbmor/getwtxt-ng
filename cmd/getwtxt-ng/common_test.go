/*
Copyright 2021 G. Benjamin Morrison

This file is part of getwtxt-ng.

getwtxt-ng is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

getwtxt-ng is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with getwtxt-ng.  If not, see <https://www.gnu.org/licenses/>.
*/
package main

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPass(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		out, err := HashPass("")
		if err != nil {
			t.Error(err.Error())
		}
		if out != "" {
			t.Errorf("Got %s, expected empty string", out)
		}
	})
	t.Run("hash a password", func(t *testing.T) {
		pass := "hunter2"
		out, err := HashPass(pass)
		if err != nil {
			t.Error(err.Error())
		}
		if err := bcrypt.CompareHashAndPassword([]byte(out), []byte(pass)); err != nil {
			t.Error(err.Error())
		}
	})
}
