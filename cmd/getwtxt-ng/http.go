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
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"

	"github.com/gbmor/getwtxt-ng/registry"
)

type APIFormat string

const (
	APIFormatPlain APIFormat = "plain"
	APIFormatJSON  APIFormat = "json"
)

func getFormat(r *http.Request) APIFormat {
	vars := mux.Vars(r)
	return APIFormat(vars["format"])
}

func getHTTPRateLimiter(conf *Config) throttled.HTTPRateLimiter {
	store, err := memstore.New(65536)
	if err != nil {
		fmt.Printf("Could not initialize memstore for HTTP rate limiter: %s", err)
		os.Exit(1)
	}

	limits := throttled.RateQuota{
		MaxRate:  throttled.PerMin(conf.ServerConfig.HTTPRequestsPerMinute),
		MaxBurst: conf.ServerConfig.HTTPRequestsBurstMax,
	}

	rl, err := throttled.NewGCRARateLimiter(store, limits)
	if err != nil {
		fmt.Printf("Couldn't build rate limiter: %s", err)
		os.Exit(1)
	}

	return throttled.HTTPRateLimiter{
		DeniedHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}),
		RateLimiter: rl,
		VaryBy:      &throttled.VaryBy{Path: true},
	}
}

func setUpRoutes(r *mux.Router, conf *Config, dbConn *registry.DB) {
	r.HandleFunc("/api/{format:json|plain}/mentions", func(w http.ResponseWriter, r *http.Request) {
		getMentionsHandler(w, r, dbConn, getFormat(r))
	}).Methods(http.MethodGet, http.MethodHead)

	r.HandleFunc("/api/{format:json|plain}/tags/{tag:[\\w]+}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		getTagsHandler(w, r, dbConn, getFormat(r), vars["tag"])
	}).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/api/{format:json|plain}/tags", func(w http.ResponseWriter, r *http.Request) {
		getTagsHandler(w, r, dbConn, getFormat(r), "")
	}).Methods(http.MethodGet, http.MethodHead)

	r.HandleFunc("/api/{format:json|plain}/tweets", func(w http.ResponseWriter, r *http.Request) {
		getTweetsHandler(w, r, dbConn, getFormat(r))
	}).Methods(http.MethodGet, http.MethodHead)

	r.HandleFunc("/api/plain/users/bulk", func(w http.ResponseWriter, r *http.Request) {
		plainBulkAddUserHandler(w, r, conf, dbConn)
	}).Methods(http.MethodPost)
	r.HandleFunc("/api/{format:json|plain}/users", func(w http.ResponseWriter, r *http.Request) {
		deleteUsersHandler(w, r, conf, dbConn, getFormat(r))
	}).Methods(http.MethodDelete)
	r.HandleFunc("/api/{format:json|plain}/users", func(w http.ResponseWriter, r *http.Request) {
		getUsersHandler(w, r, dbConn, getFormat(r))
	}).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/api/{format:json|plain}/users", func(w http.ResponseWriter, r *http.Request) {
		addUserHandler(w, r, conf, dbConn, getFormat(r))
	}).Methods(http.MethodPost)

	r.HandleFunc("/api/{format:json|plain}/version", versionHandler).
		Methods(http.MethodGet, http.MethodHead)

	r.HandleFunc("/docs/json.html", func(w http.ResponseWriter, r *http.Request) {
		jsonDocsHandler(w, r, conf, dbConn)
	}).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/docs/plain.html", func(w http.ResponseWriter, r *http.Request) {
		plainDocsHandler(w, r, conf, dbConn)
	}).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/css", func(w http.ResponseWriter, r *http.Request) {
		cssHandler(w, r, conf)
	}).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		indexHandler(w, r, conf, dbConn)
	}).Methods(http.MethodGet, http.MethodHead)
}
