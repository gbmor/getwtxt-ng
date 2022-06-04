package registry

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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gbmor/getwtxt-ng/common"
	log "github.com/sirupsen/logrus"
)

var testTwtxtFile = fmt.Sprintf(`
# this is a comment
%s	hello there
%s	this one uses rfc3339nano datetime
%d	this one uses epoch time for some reason`, time.Now().UTC().AddDate(0, 0, -10).Format(time.RFC3339),
	time.Now().UTC().AddDate(0, 0, -5).Format(time.RFC3339Nano),
	time.Now().UTC().AddDate(0, 0, -2).Unix())

var populatedDBUsers = []User{
	{
		ID:            "1",
		URL:           "https://example.com/twtxt.txt",
		Nick:          "foobar",
		PasscodeHash:  "abcdefghij0123456789",
		DateTimeAdded: time.Now().UTC().AddDate(0, 0, -15),
		LastSync:      time.Now().UTC().AddDate(0, 0, -10),
	},
	{
		ID:            "2",
		URL:           "https://example.org/twtxt.txt",
		Nick:          "barfoo",
		PasscodeHash:  "abcdefghij0123456789",
		DateTimeAdded: time.Now().UTC().AddDate(0, 0, -5),
		LastSync:      time.Now().UTC().AddDate(0, 0, -1),
	},
}

var populatedDBTweets = []Tweet{
	{
		ID:       "1",
		UserID:   "1",
		DateTime: time.Now().UTC().AddDate(0, 0, -11),
		Body:     "hallo this is dog",
	},
	{
		ID:       "2",
		UserID:   "2",
		DateTime: time.Now().UTC().AddDate(0, 0, -3),
		Body:     "oh hey there",
	},
	{
		ID:       "3",
		UserID:   "2",
		DateTime: time.Now().UTC().AddDate(0, 0, -2),
		Body:     "blah blah spam",
		Hidden:   1,
	},
}

// getDBMocker gives us the SQL DB mocker.
// We can then induce error conditions easily in tests.
// Use an in-memory SQLite DB when the data is important.
func getDBMocker(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	dbConn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err.Error())
	}

	dbWrap := DB{
		conn:              dbConn,
		EntriesPerPageMax: 1000,
		EntriesPerPageMin: 20,
		logger:            log.StandardLogger(),
	}

	return &dbWrap, mock
}

// getPopulatedDB returns an in-memory SQLite3 database with
// test data loaded into the tables.
func getPopulatedDB(t *testing.T) *DB {
	t.Helper()
	db, err := InitSQLite(":memory:", 20, 1000, nil, log.StandardLogger())
	if err != nil {
		t.Fatal(err.Error())
	}

	usersStmt := "INSERT INTO users (id, url, nick, passcode_hash, dt_added, last_sync) VALUES (?,?,?,?,?,?)"
	for _, u := range populatedDBUsers {
		u.PasscodeHash, err = common.HashPass(u.PasscodeHash)
		if err != nil {
			t.Fatal(err.Error())
		}
		if _, err := db.conn.Exec(usersStmt, u.ID, u.URL, u.Nick, u.PasscodeHash, u.DateTimeAdded.UnixNano(), u.LastSync.UnixNano()); err != nil {
			_ = db.conn.Close()
			t.Fatal(err.Error())
			return nil
		}
	}

	tweetsStmt := "INSERT INTO tweets (id, user_id, dt, body, hidden) VALUES (?,?,?,?,?)"
	for _, tw := range populatedDBTweets {
		if _, err := db.conn.Exec(tweetsStmt, tw.ID, tw.UserID, tw.DateTime.UnixNano(), tw.Body, tw.Hidden); err != nil {
			_ = db.conn.Close()
			t.Fatal(err.Error())
			return nil
		}
	}

	return db
}

func twtxtTestingHandler(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/304") {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if strings.Contains(r.URL.Path, "/404") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if strings.Contains(r.URL.Path, "/json") {
		w.Header().Set("Content-Type", common.MimeJson)
		w.WriteHeader(http.StatusOK)
		return
	}
	if strings.Contains(r.URL.Path, "/twtxt.txt") {
		w.Header().Set("Content-Type", common.MimePlain)
		_, _ = w.Write([]byte(testTwtxtFile))
		return
	}
}
