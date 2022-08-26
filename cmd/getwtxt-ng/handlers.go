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
	"encoding/json"
	"net/http"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
	log "github.com/sirupsen/logrus"
)

type JSONResponse interface {
	MessageResponse | []registry.Tweet | []registry.User
}

type MessageResponse struct {
	Message       string `json:"message"`
	Passcode      string `json:"passcode,omitempty"`
	TweetsDeleted int64  `json:"tweets_deleted,omitempty"`
	UsersDeleted  int    `json:"users_deleted,omitempty"`
}

func jsonResponseWrite[T JSONResponse](w http.ResponseWriter, body T, statusCode int) {
	jsonEncoder := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := jsonEncoder.Encode(body); err != nil {
		log.Error(err)
	}
}

func plainResponseWrite(w http.ResponseWriter, body string, statusCode int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(body)); err != nil {
		log.Error(err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request, conf *Config) {
	out := []byte("200 OK")
	w.Header().Set("Content-Type", common.MimePlain)
	if _, err := w.Write(out); err != nil {
		log.Debugf("Index Handler: %s\n", err)
	}
}
