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
	"strconv"

	"github.com/gbmor/getwtxt-ng/registry"
	log "github.com/sirupsen/logrus"
)

func getTweetsHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, format APIFormat) {
	var err error
	_ = r.ParseForm()
	pageStr := r.Form.Get("page")
	perPageStr := r.Form.Get("per_page")
	searchTerm := r.Form.Get("q")

	page := 0
	perPage := 0
	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			msg := MessageResponse{
				Message: fmt.Sprintf("Invalid page specified: %s", pageStr),
			}
			if format == APIFormatPlain {
				plainResponseWrite(w, msg.Message, http.StatusBadRequest)
			} else if format == APIFormatJSON {
				jsonResponseWrite(w, msg, http.StatusBadRequest)
			}
			return
		}
	}
	if perPageStr != "" {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil {
			msg := MessageResponse{
				Message: fmt.Sprintf("Invalid per page count specified: %s", perPageStr),
			}
			if format == APIFormatPlain {
				plainResponseWrite(w, msg.Message, http.StatusBadRequest)
			} else if format == APIFormatJSON {
				jsonResponseWrite(w, msg, http.StatusBadRequest)
			}
			return
		}
	}

	if searchTerm == "" {
		getLatestTweetsHandler(w, r, dbConn, page, perPage, format)
	} else {
		searchTweetsHandler(w, r, dbConn, page, perPage, format, searchTerm)
	}
}

func getLatestTweetsHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, page, perPage int, format APIFormat) {
	ctx := r.Context()

	tweets, err := dbConn.GetTweets(ctx, page, perPage, registry.StatusVisible)
	if err != nil {
		log.Errorf("When retrieving latest tweets, page %d, per page %d: %s", page, perPage, err)
		msg := MessageResponse{
			Message: "Internal Server Error",
		}
		if format == APIFormatPlain {
			plainResponseWrite(w, msg.Message, http.StatusInternalServerError)
		} else if format == APIFormatJSON {
			jsonResponseWrite(w, msg, http.StatusInternalServerError)
		}
		return
	}

	if format == APIFormatPlain {
		out := registry.FormatTweetsPlain(tweets)
		plainResponseWrite(w, out, http.StatusOK)
	} else if format == APIFormatJSON {
		jsonResponseWrite(w, tweets, http.StatusOK)
	}
}

func searchTweetsHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, page, perPage int, format APIFormat, searchTerm string) {
	ctx := r.Context()

	tweets, err := dbConn.SearchTweets(ctx, page, perPage, searchTerm, registry.StatusVisible)
	if err != nil {
		log.Errorf("When searching for tweets containing %s, page %d, per page %d: %s", searchTerm, page, perPage, searchTerm)
		msg := MessageResponse{
			Message: "Internal Server Error",
		}
		if format == APIFormatPlain {
			plainResponseWrite(w, msg.Message, http.StatusInternalServerError)
		} else if format == APIFormatJSON {
			jsonResponseWrite(w, msg, http.StatusInternalServerError)
		}
		return
	}

	if format == APIFormatPlain {
		out := registry.FormatTweetsPlain(tweets)
		plainResponseWrite(w, out, http.StatusOK)
	} else if format == APIFormatJSON {
		jsonResponseWrite(w, tweets, http.StatusOK)
	}
}
