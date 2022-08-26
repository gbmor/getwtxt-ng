package common

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

import (
	"crypto/rand"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func TestValidatePass(t *testing.T) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	pass := fmt.Sprintf("%x", b)
	passHash, err := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if err != nil {
		t.Fatal(err)
	}
	passHashStr := string(passHash)

	if !ValidatePass(pass, passHash) {
		t.Fatalf("Password %s didn't match hash %s", pass, passHashStr)
	}
	if ValidatePass("foo", passHash) {
		t.Fatalf("Password foo erroneously matched hash %s", passHashStr)
	}
}

func TestHashPass(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		out, err := HashPass("")
		if err != nil {
			t.Error(err.Error())
		}
		if out != nil {
			t.Errorf("Got %s, expected empty string", string(out))
		}
	})
	t.Run("hash a password", func(t *testing.T) {
		pass := "hunter2"
		out, err := HashPass(pass)
		if err != nil {
			t.Error(err.Error())
		}
		if err := bcrypt.CompareHashAndPassword(out, []byte(pass)); err != nil {
			t.Error(err.Error())
		}
	})
}

func TestIsValidURL(t *testing.T) {
	cases := []struct {
		url    string
		expect bool
	}{
		{
			url:    "https://example.com",
			expect: true,
		},
		{
			url:    "http://example.com",
			expect: true,
		},
		{
			url: "gopher://example.com",
		},
		{
			url: "",
		},
		{
			url: "http://localhost",
		},
		{
			url: "http://127.0.0.1",
		},
		{
			url: "http://::1",
		},
	}
	for _, tt := range cases {
		t.Run(tt.url, func(t *testing.T) {
			out := IsValidURL(tt.url, log.StandardLogger())
			if out != tt.expect {
				t.Errorf("Got %v, expected %v", out, tt.expect)
			}
		})
	}
}
