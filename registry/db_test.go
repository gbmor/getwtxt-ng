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
	"log"
	"reflect"
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
	dbConn, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
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
		mock.ExpectQuery("SELECT * FROM users WHERE url = ?").
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

func TestDB_InsertUser(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	memDB := getPopulatedDB(t)

	testUser := User{
		URL:  "https://example.net/twtxt.txt",
		Nick: "foobaz",
	}
	insertStmt := "INSERT INTO users (url, nick, dt_added, last_sync) VALUES(?,?,?, 0)"

	t.Run("invalid params provided", func(t *testing.T) {
		db := DB{}
		err := db.InsertUser(nil)
		if err == nil {
			t.Error("Expected error, but got nil")
		}
		if !strings.Contains(err.Error(), "incomplete user info") {
			t.Errorf("Expected incomplete user info error, got: %s", err)
		}
	})

	t.Run("error beginning tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.InsertUser(&testUser)
		if !xerrors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to insert user, tx done", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(insertStmt).
			WithArgs(testUser.URL, testUser.Nick, sqlmock.AnyArg()).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertUser(&testUser)
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("fail to insert user, dupe", func(t *testing.T) {
		err := memDB.InsertUser(&populatedDBUsers[0])
		if err == nil {
			t.Error("Expected error inserting duplicate user, got nil")
		}
	})

	t.Run("insert new user", func(t *testing.T) {
		err := memDB.InsertUser(&testUser)
		if err != nil {
			t.Error(err.Error())
		}
		getUser := "SELECT * FROM users WHERE url = ?"
		dbUser := User{}
		dt := int64(0)
		err = memDB.conn.QueryRow(getUser, testUser.URL).Scan(&dbUser.ID, &dbUser.URL, &dbUser.Nick, &dt, &dt)
		if err != nil {
			t.Error(err.Error())
		}
		testUser.DateTimeAdded = time.Time{}
		testUser.LastSync = time.Time{}
		testUser.ID = dbUser.ID
		if !reflect.DeepEqual(testUser, dbUser) {
			t.Errorf("Expected:\n%#v\nGot:\n%#v\n", testUser, dbUser)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_DeleteUser(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	delTweetsStmt := "DELETE FROM tweets WHERE user_id = ?"
	delUserStmt := "DELETE FROM users WHERE id = ?"

	t.Run("invalid user info", func(t *testing.T) {
		emptyUser := User{}
		_, err := mockDB.DeleteUser(&emptyUser)
		if !strings.Contains(err.Error(), "invalid user provided") {
			t.Errorf("Expected invalid user error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		_, err := mockDB.DeleteUser(&populatedDBUsers[0])
		if !xerrors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to delete tweets", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(delTweetsStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		_, err := mockDB.DeleteUser(&populatedDBUsers[0])
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("fail to delete user", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(delTweetsStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnResult(sqlmock.NewResult(23, 7))
		mock.ExpectExec(delUserStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		_, err := mockDB.DeleteUser(&populatedDBUsers[0])
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("error on commit", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(delTweetsStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnResult(sqlmock.NewResult(23, 7))
		mock.ExpectExec(delUserStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnResult(sqlmock.NewResult(23, 7))
		mock.ExpectCommit().
			WillReturnError(sql.ErrTxDone)
		_, err := mockDB.DeleteUser(&populatedDBUsers[0])
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("successful", func(t *testing.T) {
		tweets, err := memDB.DeleteUser(&populatedDBUsers[0])
		if err != nil {
			t.Error(err.Error())
		}
		if tweets != 1 {
			t.Errorf("Expected 1 tweet removed, got %d removed", tweets)
		}

		getUserStmt := "SELECT url FROM users WHERE id = ?"
		rows, err := memDB.conn.Query(getUserStmt, populatedDBUsers[0].ID)
		if err != nil {
			t.Error(err.Error())
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			userUrl := ""
			err := rows.Scan(&userUrl)
			if !xerrors.Is(err, sql.ErrNoRows) {
				t.Errorf("Expected row to be missing? %s", err)
			}
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_InsertTweets(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	insertStmt := "INSERT INTO tweets (user_id, dt, body) VALUES(?,?,?)"

	t.Run("no tweets provided", func(t *testing.T) {
		err := mockDB.InsertTweets(nil)
		if !strings.Contains(err.Error(), "invalid tweets provided") {
			t.Errorf("Expected invalid tweets error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.InsertTweets(populatedDBTweets)
		if !xerrors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to prepare stmt", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectPrepare(insertStmt).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertTweets(populatedDBTweets)
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("fail to insert tweets", func(t *testing.T) {
		mock.ExpectBegin()
		stmt := mock.ExpectPrepare(insertStmt)
		stmt.ExpectExec().
			WithArgs(populatedDBTweets[0].ID, populatedDBTweets[0].DateTime, populatedDBTweets[0].Body).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertTweets(populatedDBTweets)
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("insert tweets", func(t *testing.T) {
		err := memDB.InsertTweets(populatedDBTweets)
		if err != nil {
			t.Error(err.Error())
		}
		outTweets := make([]Tweet, 0)
		getAllTweets := "SELECT * FROM tweets"
		rows, err := memDB.conn.Query(getAllTweets)
		if err != nil {
			t.Error(err.Error())
		}
		defer func() {
			_ = rows.Close()
		}()
		count := 0
		for rows.Next() {
			dt := int64(0)
			thisTweet := Tweet{}
			err := rows.Scan(&thisTweet.ID, &thisTweet.UserID, &dt, &thisTweet.Body, &thisTweet.Hidden)
			if err != nil {
				t.Error(err.Error())
			}
			thisTweet.DateTime = populatedDBTweets[count].DateTime
			outTweets = append(outTweets, thisTweet)
			count++
		}
		if !reflect.DeepEqual(outTweets, populatedDBTweets) {
			t.Errorf("Expected:\n%#v\nGot:\n%#v\n", populatedDBTweets, outTweets)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_ToggleTweetHiddenStatus(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	toggleStmt := "UPDATE tweets SET hidden = ? WHERE user_id = ? AND dt = ?"

	t.Run("invalid params", func(t *testing.T) {
		err := mockDB.ToggleTweetHiddenStatus("", time.Time{}, StatusHidden)
		if !strings.Contains(err.Error(), "invalid user ID") {
			t.Errorf("Expected invalid params error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.ToggleTweetHiddenStatus(populatedDBTweets[0].ID, populatedDBTweets[0].DateTime, StatusHidden)
		if !xerrors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to toggle status", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(toggleStmt).
			WithArgs(StatusHidden, populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime.Unix()).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.ToggleTweetHiddenStatus(populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime, StatusHidden)
		if !xerrors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("switch tweet visibility", func(t *testing.T) {
		err := memDB.ToggleTweetHiddenStatus(populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime, StatusHidden)
		if err != nil {
			t.Error(err.Error())
		}
		getVisibilityStmt := "SELECT hidden FROM tweets WHERE body = ?"
		hidden := 0
		row := memDB.conn.QueryRow(getVisibilityStmt, populatedDBTweets[0].Body)
		if err := row.Scan(&hidden); err != nil {
			t.Error(err.Error())
		}
		if TweetVisibilityStatus(hidden) != StatusHidden {
			t.Errorf("Expected %d, got %d", StatusHidden, hidden)
		}

		// Dump the tweets table to stderr if something went wrong
		if t.Failed() {
			getTweets := "SELECT * FROM tweets"
			rows, err := memDB.conn.Query(getTweets)
			if err != nil {
				t.Error(err.Error())
			}
			defer func() {
				_ = rows.Close()
			}()
			for rows.Next() {
				thisTweet := Tweet{}
				dt := int64(0)
				err := rows.Scan(&thisTweet.ID, &thisTweet.UserID, &dt, &thisTweet.Body, &thisTweet.Hidden)
				if err != nil {
					t.Error(err.Error())
				}
				thisTweet.DateTime = time.Unix(dt, 0)
				log.Printf("%#v\n", thisTweet)
			}
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}
