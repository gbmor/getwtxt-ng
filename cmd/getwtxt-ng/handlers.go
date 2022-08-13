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
	"strings"
	"time"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
	log "github.com/sirupsen/logrus"
)

func indexHandler(w http.ResponseWriter, r *http.Request, conf *Config) {
	out := []byte("200 OK")
	w.Header().Set("Content-Type", common.MimePlain)
	if _, err := w.Write(out); err != nil {
		log.Debugf("Index Handler: %s\n", err)
	}
}

func addUserHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseForm()
	nick := strings.TrimSpace(r.Form.Get("nick"))
	twtxtURL := strings.TrimSpace(r.Form.Get("url"))

	if nick == "" {
		http.Error(w, "Please provide a nickname", http.StatusBadRequest)
		return
	}
	if twtxtURL == "" {
		http.Error(w, "Please provide a twtxt.txt URL", http.StatusBadRequest)
		return
	}

	userSearchOut, err := dbConn.SearchUsers(ctx, 1, conf.ServerConfig.EntriesPerPageMin, twtxtURL)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Errorf("While searching for user %s: %s", twtxtURL, err)
		return
	}
	if len(userSearchOut) > 0 {
		http.Error(w, "Cannot add duplicate user", http.StatusBadRequest)
		return
	}

	user := registry.User{
		Nick: nick,
		URL:  twtxtURL,
	}

	passcode, err := user.GeneratePasscode()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Errorf("While generating passcode for new user %s %s: %s", user.Nick, user.URL, err)
		return
	}

	if err := dbConn.InsertUser(ctx, &user); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Errorf("When adding new user %s %s: %s", user.Nick, user.URL, err)
		return
	}

	response := fmt.Sprintf("You have been added! Your user's generated passcode is: %s\n", passcode)

	tweets, err := dbConn.FetchTwtxt(twtxtURL, user.ID, time.Time{})
	if err != nil {
		log.Errorf("When fetching twtxt.txt for new user %s %s: %s", user.Nick, user.URL, err)
		response = fmt.Sprintf("%sHowever, we were unable to fetch your twtxt file.", response)
		http.Error(w, response, http.StatusInternalServerError)
		return
	}

	if len(tweets) > 0 {
		if err := dbConn.InsertTweets(ctx, tweets); err != nil {
			log.Errorf("When adding tweets for new user %s %s: %s", user.Nick, user.URL, err)
			response = fmt.Sprintf("%sHowever, we were unable to add your tweets to the registry for some reason.", response)
			http.Error(w, response, http.StatusInternalServerError)
			return
		}
	}

	if _, err := w.Write([]byte(response)); err != nil {
		log.Error(err)
	}
}
