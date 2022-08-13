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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
	log "github.com/sirupsen/logrus"
)

type JSONResponse interface {
	addUserRespJSON
}

type addUserRespJSON struct {
	Message  string `json:"message"`
	Passcode string `json:"passcode,omitempty"`
}

func jsonResponseWrite[T JSONResponse](w http.ResponseWriter, body T, statusCode int) {
	jsonEncoder := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := jsonEncoder.Encode(body); err != nil {
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

func plainAddUserHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/plain")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseForm()
	nick := strings.TrimSpace(r.Form.Get("nickname"))
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

func jsonAddUserHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)
	ctx := r.Context()
	response := addUserRespJSON{}

	if r.Method != http.MethodPost {
		response.Message = "Method Not Allowed"
		jsonResponseWrite(w, response, http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		response.Message = "Internal Server Error"
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}

	user := registry.User{}
	if err := json.Unmarshal(bodyBytes, &user); err != nil {
		log.Error(err)
		response.Message = "Invalid Request Body"
		jsonResponseWrite(w, response, http.StatusBadRequest)
		return
	}

	if user.Nick == "" {
		response.Message = "Please provide a nickname"
		jsonResponseWrite(w, response, http.StatusBadRequest)
		return
	}
	if user.URL == "" {
		response.Message = "Please provide a twtxt.txt URL"
		jsonResponseWrite(w, response, http.StatusBadRequest)
		return
	}

	userSearchOut, err := dbConn.SearchUsers(ctx, 1, conf.ServerConfig.EntriesPerPageMin, user.URL)
	if err != nil {
		log.Errorf("While searching for user %s: %s", user.URL, err)
		response.Message = "Internal Server Error"
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}
	if len(userSearchOut) > 0 {
		response.Message = "Cannot add duplicate user"
		jsonResponseWrite(w, response, http.StatusBadRequest)
		return
	}

	passcode, err := user.GeneratePasscode()
	if err != nil {
		log.Errorf("While generating passcode for new user %s %s: %s", user.Nick, user.URL, err)
		response.Message = "Internal Server Error"
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}

	if err := dbConn.InsertUser(ctx, &user); err != nil {
		log.Errorf("When adding new user %s %s: %s", user.Nick, user.URL, err)
		response.Message = "Internal Server Error"
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}

	response.Message = "You have been added and your passcode has been generated."
	response.Passcode = passcode

	tweets, err := dbConn.FetchTwtxt(user.URL, user.ID, time.Time{})
	if err != nil {
		log.Errorf("When fetching twtxt.txt for new user %s %s: %s", user.Nick, user.URL, err)
		response.Message = fmt.Sprintf("%s However, we were unable to fetch your twtxt file at %s. Another attempt will be made at the next sync interval (every %s)",
			user.URL, conf.ServerConfig.FetchInterval, response.Message)
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}

	if len(tweets) > 0 {
		if err := dbConn.InsertTweets(ctx, tweets); err != nil {
			log.Errorf("When adding tweets for new user %s %s: %s", user.Nick, user.URL, err)
			response.Message = fmt.Sprintf("%s However, we were unable to add your tweets to the registry for some reason. Please contact the administrator of this instance.", response)
			jsonResponseWrite(w, response, http.StatusInternalServerError)
			return
		}
	}

	jsonResponseWrite(w, response, http.StatusOK)
}
