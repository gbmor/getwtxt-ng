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

import "golang.org/x/crypto/bcrypt"

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
