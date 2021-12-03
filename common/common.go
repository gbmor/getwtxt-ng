// Package common implements utility functions shared between other packages.
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
	"bytes"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

const MimePlain = "text/plain; charset=utf-8"

// HashPass returns the bcrypt hash of the provided string.
// If an empty string is provided, return an empty string.
func HashPass(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	h, err := bcrypt.GenerateFromPassword([]byte(s), 14)
	if err != nil {
		return "", err
	}

	return string(h), nil
}

// HttpWriteLn writes a line to the provided http.ResponseWriter with an appended newline.
// Sends the provided HTTP status code.
func HttpWriteLn(in []byte, w http.ResponseWriter, code int, mime string) error {
	if !bytes.HasSuffix(in, []byte("\n")) {
		in = append(in, byte('\n'))
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(code)
	_, err := w.Write(in)

	return err
}
