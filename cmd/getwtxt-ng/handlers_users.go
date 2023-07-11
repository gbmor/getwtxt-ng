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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
)

func addUserHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB, format APIFormat) {
	switch format {
	case APIFormatPlain:
		plainAddUserHandler(w, r, conf, dbConn)
	case APIFormatJSON:
		jsonAddUserHandler(w, r, conf, dbConn)
	default:
		// should have 404'ed before this
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

func plainBulkAddUserHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	log.SetLevel(log.ErrorLevel)
	defer log.SetLevel(log.InfoLevel)
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/plain")
	_ = r.ParseForm()
	remoteURL := r.Form.Get("source")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	auth := r.Header.Get("X-Auth")
	if auth == "" {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	if !common.ValidatePass(auth, []byte(conf.ServerConfig.AdminPassword)) {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	if !common.IsValidURL(remoteURL, log.StandardLogger()) {
		msg := fmt.Sprintf("400 Bad Request: couldn't parse %s as URL", remoteURL)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		log.Errorf("Couldn't create http request to fetch list of new users from %s: %s", remoteURL, err)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	resp, err := dbConn.Client.Do(req)
	if err != nil {
		log.Errorf("Couldn't fetch list of new users from %s: %s", remoteURL, err)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	usersToAdd := make([]registry.User, 0, 5)

	bodyScanner := bufio.NewScanner(resp.Body)
	for bodyScanner.Scan() {
		line := bodyScanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// This is to prevent variations of the same URL showing up multiple times.
		// Eg: http://example.com/twtxt.txt vs https://example.com/twtxt.txt
		// We're also chomping www. off.
		parsedURL, err := url.Parse(fields[1])
		if err != nil {
			log.Errorf("couldn't parse %s as URL: %s", fields[1], err)
			continue
		}
		host := strings.TrimPrefix(parsedURL.Host, "www.")
		constructedURL := fmt.Sprintf("%s%s", host, parsedURL.Path)

		userSearchOut, err := dbConn.SearchUsers(ctx, 1, conf.ServerConfig.EntriesPerPageMin, constructedURL)
		if err != nil {
			log.Errorf("While searching for user %s: %s", fields[1], err)
			continue
		}
		if len(userSearchOut) > 0 {
			continue
		}
		var dt time.Time
		if len(fields) < 3 {
			dt = time.Now().UTC()
		} else {
			dt, err = time.Parse(time.RFC3339, fields[2])
			if err != nil {
				dt, err = time.Parse(time.RFC3339Nano, fields[2])
				if err != nil {
					continue
				}
			}
		}

		thisUser := registry.User{
			Nick:          fields[0],
			URL:           fields[1],
			DateTimeAdded: dt,
		}
		usersToAdd = append(usersToAdd, thisUser)
	}

	users, err := dbConn.InsertUsers(ctx, usersToAdd)
	if err != nil {
		log.Errorf("When bulk inserting users: %s", err)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	for i, user := range users {
		tweets, err := dbConn.FetchTwtxt(user.URL, user.ID, time.Time{})
		if err != nil {
			log.Errorf("Couldn't fetch tweets for %s: %s", user.URL, err)
			continue
		}
		err = dbConn.InsertTweets(ctx, tweets)
		if err != nil {
			log.Errorf("Couldn't fetch tweets for %s: %s", user.URL, err)
			continue
		}
		users[i].LastSync = time.Now().UTC()
	}

	plainUsersResp := registry.FormatUsersPlain(users)
	plainResponseWrite(w, plainUsersResp, http.StatusOK)
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

	// This is to prevent variations of the same URL showing up multiple times.
	// Eg: http://example.com/twtxt.txt vs https://example.com/twtxt.txt
	// We're also chomping www. off.
	parsedURL, err := url.Parse(twtxtURL)
	if err != nil {
		msg := "400 Bad Request: Invalid URL"
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	host := strings.TrimPrefix(parsedURL.Host, "www.")
	constructedURL := fmt.Sprintf("%s%s", host, parsedURL.Path)

	userSearchOut, err := dbConn.SearchUsers(ctx, 1, conf.ServerConfig.EntriesPerPageMin, constructedURL)
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
		if errors.Is(err, registry.ErrUserURLIsNotTwtxtFile) || errors.Is(err, registry.ErrIncompleteUserInfo) {
			msg := "400 Bad Request: Make sure the info provided is valid and the URL points to a twtxt.txt file"
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
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
	response := MessageResponse{}

	if r.Method != http.MethodPost {
		response.Message = "Method Not Allowed"
		jsonResponseWrite(w, response, http.StatusMethodNotAllowed)
		return
	}

	bodyDecoder := json.NewDecoder(r.Body)

	user := registry.User{}
	if err := bodyDecoder.Decode(&user); err != nil {
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

	// This is to prevent variations of the same URL showing up multiple times.
	// Eg: http://example.com/twtxt.txt vs https://example.com/twtxt.txt
	// We're also chomping www. off.
	parsedURL, err := url.Parse(user.URL)
	if err != nil {
		response.Message = "400 Bad Request: Invalid URL"
		jsonResponseWrite(w, response, http.StatusBadRequest)
	}
	host := strings.TrimPrefix(parsedURL.Host, "www.")
	constructedURL := fmt.Sprintf("%s%s", host, parsedURL.Path)

	userSearchOut, err := dbConn.SearchUsers(ctx, 1, conf.ServerConfig.EntriesPerPageMin, constructedURL)
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
		if errors.Is(err, registry.ErrUserURLIsNotTwtxtFile) || errors.Is(err, registry.ErrIncompleteUserInfo) {
			response.Message = "400 Bad Request: Make sure the info provided is valid and the URL points to a twtxt.txt file"
			jsonResponseWrite(w, response, http.StatusBadRequest)
			return
		}
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
			response.Message, user.URL, conf.ServerConfig.FetchInterval)
		jsonResponseWrite(w, response, http.StatusInternalServerError)
		return
	}

	if len(tweets) > 0 {
		if err := dbConn.InsertTweets(ctx, tweets); err != nil {
			log.Errorf("When adding tweets for new user %s %s: %s", user.Nick, user.URL, err)
			response.Message = fmt.Sprintf("%s However, we were unable to add your tweets to the registry for some reason. Please contact the administrator of this instance.", response.Message)
			jsonResponseWrite(w, response, http.StatusInternalServerError)
			return
		}
	}

	jsonResponseWrite(w, response, http.StatusOK)
}

func getUsersHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, format APIFormat) {
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
		getLatestUsersHandler(w, r, dbConn, page, perPage, format)
	} else {
		searchUsersHandler(w, r, dbConn, page, perPage, format, searchTerm)
	}
}

func getLatestUsersHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, page, perPage int, format APIFormat) {
	ctx := r.Context()

	users, err := dbConn.GetUsers(ctx, page, perPage)
	if err != nil {
		log.Errorf("When retrieving latest users, page %d, per page %d: %s", page, perPage, err)
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
		out := registry.FormatUsersPlain(users)
		plainResponseWrite(w, out, http.StatusOK)
	} else if format == APIFormatJSON {
		jsonResponseWrite(w, users, http.StatusOK)
	}
}

func searchUsersHandler(w http.ResponseWriter, r *http.Request, dbConn *registry.DB, page, perPage int, format APIFormat, searchTerm string) {
	ctx := r.Context()

	users, err := dbConn.SearchUsers(ctx, page, perPage, searchTerm)
	if err != nil {
		log.Errorf("When retrieving latest users, page %d, per page %d: %s", page, perPage, err)
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
		out := registry.FormatUsersPlain(users)
		plainResponseWrite(w, out, http.StatusOK)
	} else if format == APIFormatJSON {
		jsonResponseWrite(w, users, http.StatusOK)
	}
}

func deleteUsersHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB, format APIFormat) {
	switch format {
	case APIFormatPlain:
		plainDeleteUsersHandler(w, r, conf, dbConn)
	case APIFormatJSON:
		jsonDeleteUsersHandler(w, r, conf, dbConn)
	default:
		// should have 404'ed before this
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

func plainDeleteUsersHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	ctx := r.Context()
	_ = r.ParseForm()

	pass := r.Header.Get("X-Auth")
	if pass == "" {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	isAdmin := common.ValidatePass(pass, []byte(conf.ServerConfig.AdminPassword))

	urls := r.Form["url"]
	if len(urls) < 1 || urls[0] == "" {
		http.Error(w, "400 Bad Request: No user(s) to delete", http.StatusBadRequest)
		return
	}

	if !isAdmin {
		if len(urls) > 1 {
			http.Error(w, "403 Forbidden: Non-admin users may only delete themselves", http.StatusForbidden)
			return
		}

		dbUser, err := dbConn.GetFullUserByURL(ctx, urls[0])
		if err != nil {
			log.Errorf("When grabbing user %s: %s", urls[0], err)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !common.ValidatePass(pass, dbUser.PasscodeHash) {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		nTweets, err := dbConn.DeleteUser(ctx, dbUser)
		if err != nil {
			log.Errorf("When deleting user %s: %s", dbUser.URL, err)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}

		out := fmt.Sprintf("Deleted user %s\nDeleted %d tweets\n", dbUser.URL, nTweets)
		if _, err := w.Write([]byte(out)); err != nil {
			log.Error(err)
		}

		return
	}

	tweetCount, err := dbConn.DeleteUsers(ctx, urls)
	if err != nil {
		log.Errorf("When deleting %d users: %s", len(urls), err)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	out := fmt.Sprintf("Deleted %d users\nDeleted %d tweets\n", len(urls), tweetCount)
	if _, err := w.Write([]byte(out)); err != nil {
		log.Error(err)
	}
}

func jsonDeleteUsersHandler(w http.ResponseWriter, r *http.Request, conf *Config, dbConn *registry.DB) {
	ctx := r.Context()

	pass := r.Header.Get("X-Auth")
	if pass == "" {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	isAdmin := common.ValidatePass(pass, []byte(conf.ServerConfig.AdminPassword))

	bodyDecoder := json.NewDecoder(r.Body)

	users := make([]registry.User, 0, 2)
	if err := bodyDecoder.Decode(&users); err != nil {
		msg := MessageResponse{
			Message: "400 Bad Request: Invalid request body",
		}
		jsonResponseWrite(w, msg, http.StatusBadRequest)
		return
	}

	if len(users) < 1 {
		msg := MessageResponse{
			Message: "400 Bad Request: No user(s) to delete",
		}
		jsonResponseWrite(w, msg, http.StatusBadRequest)
		return
	}

	if !isAdmin {
		if len(users) > 1 {
			msg := MessageResponse{
				Message: "403 Forbidden: Non-admin users may only delete themselves",
			}
			jsonResponseWrite(w, msg, http.StatusForbidden)
			return
		}
		firstUserURL := users[0].URL

		dbUser, err := dbConn.GetFullUserByURL(ctx, firstUserURL)
		if err != nil {
			log.Errorf("When grabbing user %s: %s", firstUserURL, err)
			msg := MessageResponse{
				Message: "500 Internal Server Error",
			}
			jsonResponseWrite(w, msg, http.StatusInternalServerError)
			return
		}

		if !common.ValidatePass(pass, dbUser.PasscodeHash) {
			msg := MessageResponse{
				Message: "403 Forbidden",
			}
			jsonResponseWrite(w, msg, http.StatusForbidden)
			return
		}

		nTweets, err := dbConn.DeleteUser(ctx, dbUser)
		if err != nil {
			msg := MessageResponse{
				Message: fmt.Sprintf("When deleting user %s: %s", dbUser.URL, err),
			}
			jsonResponseWrite(w, msg, http.StatusInternalServerError)
			return
		}

		msg := MessageResponse{
			Message:       fmt.Sprintf("Deleted user %s", dbUser.URL),
			TweetsDeleted: nTweets,
		}
		jsonResponseWrite(w, msg, http.StatusOK)

		return
	}

	urls := make([]string, 0, len(users))
	for _, user := range users {
		urls = append(urls, user.URL)
	}

	nTweets, err := dbConn.DeleteUsers(ctx, urls)
	if err != nil {
		msg := MessageResponse{
			Message: "500 Internal Server Error",
		}
		jsonResponseWrite(w, msg, http.StatusInternalServerError)
		return
	}

	msg := MessageResponse{
		Message:       "Deleted users successfully",
		UsersDeleted:  len(users),
		TweetsDeleted: nTweets,
	}
	jsonResponseWrite(w, msg, http.StatusOK)
}
