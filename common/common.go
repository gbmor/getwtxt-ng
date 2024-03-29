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
	"net"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const MimePlain = "text/plain; charset=utf-8"
const MimeJson = "application/json; charset=utf-8"

var Version = "trunk"

// HashPass returns the bcrypt hash of the provided string.
// If an empty string is provided, return an empty string.
func HashPass(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	h, err := bcrypt.GenerateFromPassword([]byte(s), 12)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// ValidatePass returns true if the password matches the bcrypt hash, false otherwise.
func ValidatePass(pass string, hash []byte) bool {
	return bcrypt.CompareHashAndPassword(hash, []byte(pass)) == nil
}

// IsValidURL returns true if the provided URL is a valid-looking HTTP or HTTPS URL.
func IsValidURL(destURL string, logger *log.Logger) bool {
	if strings.TrimSpace(destURL) == "" {
		return false
	}
	parsedURL, err := url.Parse(destURL)
	if err != nil {
		logger.Debugf("Error parsing URL %s: %s", destURL, err)
		return false
	}

	if parsedURL.Host == "localhost" {
		return false
	}
	ip := net.ParseIP(parsedURL.Host)
	if ip.IsLoopback() {
		return false
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return false
	}
	return true
}
