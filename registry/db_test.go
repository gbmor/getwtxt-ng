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
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/xerrors"
)

var populatedDBUsers = []User{
	{
		ID:            "1",
		URL:           "https://example.com/twtxt.txt",
		Nick:          "foobar",
		DateTimeAdded: time.Now().UTC().AddDate(0, 0, -15),
		LastSync:      time.Now().UTC().AddDate(0, 0, -10),
	},
	{
		ID:            "2",
		URL:           "https://example.org/twtxt.txt",
		Nick:          "barfoo",
		DateTimeAdded: time.Now().UTC().AddDate(0, 0, -5),
		LastSync:      time.Now().UTC().AddDate(0, 0, -1),
	},
}

var populatedDBTweets = []Tweet{
	{
		ID:       "1",
		UserID:   "1",
		DateTime: time.Now().UTC().AddDate(0, 0, -11),
		Body:     "hello world",
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
		Body:     "blah blah spam post",
		Hidden:   1,
	},
}

// getDBMocker gives us the SQL DB mocker.
// We can then induce error conditions easily in tests.
// Use an in-memory SQLite DB when the data is important.
func getDBMocker(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	dbConn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err.Error())
	}

	return &DB{conn: dbConn}, mock
}

// getPopulatedDB returns an in-memory SQLite3 database with
// test data loaded into the tables.
func getPopulatedDB(t *testing.T) *DB {
	t.Helper()
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err.Error())
	}

	usersStmt := "INSERT INTO users (id, url, nick, dt_added, last_sync) VALUES (?,?,?,?,?)"
	for _, u := range populatedDBUsers {
		if _, err := db.conn.Exec(usersStmt, u.ID, u.URL, u.Nick, u.DateTimeAdded.Unix(), u.LastSync.Unix()); err != nil {
			_ = db.conn.Close()
			t.Fatal(err.Error())
			return nil
		}
	}

	tweetsStmt := "INSERT INTO tweets (id, user_id, dt, body, hidden) VALUES (?,?,?,?,?)"
	for _, tw := range populatedDBTweets {
		if _, err := db.conn.Exec(tweetsStmt, tw.ID, tw.UserID, tw.DateTime.Unix(), tw.Body, tw.Hidden); err != nil {
			_ = db.conn.Close()
			t.Fatal(err.Error())
			return nil
		}
	}

	return db
}

func TestInitDB(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Error(err.Error())
	}
	defer func() {
		if err := db.conn.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	t.Run("in-memory, check for tables", func(t *testing.T) {
		rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
		if err != nil {
			t.Error(err.Error())
		}
		tables := make([]string, 0)
		for rows.Next() {
			tbl := ""
			if err := rows.Scan(&tbl); err != nil {
				t.Error(err.Error())
			}
			tables = append(tables, tbl)
		}
		if tables[1] != "tweets" && tables[2] != "users" {
			t.Errorf("Got unexpected table names: %v", tables)
		}
	})
}

func TestDB_GetUserByURL(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	memDB := getPopulatedDB(t)
	defer func() {
		if err := memDB.conn.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	t.Run("invalid user URL", func(t *testing.T) {
		db := DB{}
		_, err := db.GetUserByURL("    ")
		if err == nil {
			t.Error("expected error, got nil")
			return
		}
		if !strings.Contains(err.Error(), "empty user URL provided") {
			t.Errorf("expected empty URL error, got: %s", err)
		}
	})

	t.Run("couldn't retrieve user", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE url = ?").
			WithArgs("https://example.net/twtxt.txt").
			WillReturnError(sql.ErrNoRows)
		_, err := mockDB.GetUserByURL("https://example.net/twtxt.txt")
		if !xerrors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("get a user successfully", func(t *testing.T) {
		out, err := memDB.GetUserByURL("https://example.com/twtxt.txt")
		if err != nil {
			t.Error(err.Error())
		}
		if out.Nick != "foobar" {
			t.Errorf("Expected nick 'foobar', got '%s'", out.Nick)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}
