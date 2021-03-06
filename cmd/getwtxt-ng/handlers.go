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
	"net/http"

	"github.com/gbmor/getwtxt-ng/common"
	log "github.com/sirupsen/logrus"
)

func indexHandler(w http.ResponseWriter, r *http.Request, conf *Config) {
	out := []byte("200 OK")
	w.Header().Set("Content-Type", common.MimePlain)
	if _, err := w.Write(out); err != nil {
		log.Debugf("Index Handler: %s\n", err)
	}
}
