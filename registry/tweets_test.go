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
	"context"
	"database/sql"
	"errors"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDB_InsertTweets(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	insertStmt := "INSERT OR IGNORE INTO tweets (user_id, dt, body, contains_mentions, contains_tags) VALUES(?,?,?,?,?)"

	t.Run("no tweets provided", func(t *testing.T) {
		err := mockDB.InsertTweets(ctx, nil)
		if !strings.Contains(err.Error(), "invalid tweets provided") {
			t.Errorf("Expected invalid tweets error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.InsertTweets(ctx, populatedDBTweets)
		if !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to prepare stmt", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectPrepare(insertStmt).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertTweets(ctx, populatedDBTweets)
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("fail to insert tweets", func(t *testing.T) {
		mock.ExpectBegin()
		stmt := mock.ExpectPrepare(insertStmt)
		stmt.ExpectExec().
			WithArgs(populatedDBTweets[0].ID, populatedDBTweets[0].DateTime.UnixNano(), populatedDBTweets[0].Body, 0, 0).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertTweets(ctx, populatedDBTweets)
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("insert tweets", func(t *testing.T) {
		err := memDB.InsertTweets(ctx, populatedDBTweets)
		if err != nil {
			t.Error(err.Error())
		}
		outTweets := make([]Tweet, 0)
		getAllTweets := "SELECT id, user_id, dt, body, hidden FROM tweets"
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

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := memDB.InsertTweets(ctx, populatedDBTweets)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_ToggleTweetHiddenStatus(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	toggleStmt := "UPDATE tweets SET hidden = ? WHERE user_id = ? AND dt = ?"

	t.Run("invalid params", func(t *testing.T) {
		err := mockDB.ToggleTweetHiddenStatus(ctx, "", time.Time{}, StatusHidden)
		if !strings.Contains(err.Error(), "invalid user ID") {
			t.Errorf("Expected invalid params error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.ToggleTweetHiddenStatus(ctx, populatedDBTweets[0].ID, populatedDBTweets[0].DateTime, StatusHidden)
		if !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to toggle status", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(toggleStmt).
			WithArgs(StatusHidden, populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime.UnixNano()).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.ToggleTweetHiddenStatus(ctx, populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime, StatusHidden)
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("switch tweet visibility", func(t *testing.T) {
		err := memDB.ToggleTweetHiddenStatus(ctx, populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime, StatusHidden)
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
			getTweets := "SELECT id, user_id, dt, body, hidden FROM tweets"
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
				thisTweet.DateTime = time.Unix(0, dt)
				log.Printf("%#v\n", thisTweet)
			}
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := memDB.ToggleTweetHiddenStatus(ctx, populatedDBTweets[0].UserID, populatedDBTweets[0].DateTime, StatusHidden)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_GetTweets(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()

	visibleCount := 0
	for _, populatedTweet := range populatedDBTweets {
		if populatedTweet.Hidden == 0 {
			visibleCount++
		}
	}

	tweetStmt := `SELECT id, user_id, nick, url, dt, body, hidden
					FROM (SELECT tweets.*, users.nick AS nick, users.url AS url, ROW_NUMBER() OVER (ORDER BY dt DESC) AS set_id
					      FROM tweets LEFT JOIN users ON users.id = tweets.user_id WHERE tweets.hidden = ?)
					WHERE set_id > ?
  					AND set_id <= ?`

	t.Run("error on query", func(t *testing.T) {
		mock.ExpectQuery(tweetStmt).
			WithArgs(StatusVisible, 0, 20).
			WillReturnError(sql.ErrNoRows)
		_, err := mockDB.GetTweets(ctx, -1, 2, StatusVisible)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("fail to scan", func(t *testing.T) {
		mock.ExpectQuery(tweetStmt).
			WithArgs(StatusVisible, 0, 1000).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "user_id", "dt", "body", "hidden"}).
					AddRow("1", "2", "thirty five o'clock", "hello there", 0))
		out, err := mockDB.GetTweets(ctx, 0, 2000, StatusVisible)
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) > 0 {
			t.Errorf("Got %d tweets, expected zero", len(out))
		}
	})

	t.Run("get tweets", func(t *testing.T) {
		out, err := memDB.GetTweets(ctx, 0, 20, StatusVisible)
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) != visibleCount {
			t.Errorf("Expected %d tweets, got %d", len(populatedDBTweets), len(out))
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := memDB.GetTweets(ctx, 0, 20, StatusVisible)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestDB_SearchTweets(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	searchStmt := `SELECT id, user_id, nick, url, dt, body, hidden
					FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY dt DESC) AS set_id
					      FROM tweets_search WHERE tweets_search.hidden = ? AND body MATCH ?)
					WHERE set_id > ? AND set_id <= ?`

	t.Run("fail to query", func(t *testing.T) {
		mock.ExpectQuery(searchStmt).
			WithArgs(StatusVisible, "foo", 0, 20).
			WillReturnError(sql.ErrNoRows)
		_, err := mockDB.SearchTweets(ctx, 1, 1, "foo", StatusVisible)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("fail to scan", func(t *testing.T) {
		mock.ExpectQuery(searchStmt).
			WithArgs(StatusVisible, "foo", 0, 1000).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "user_id", "dt", "body", "hidden"}).
					AddRow("1", "2", "thirty five o'clock", "hello there", 0))
		out, err := mockDB.SearchTweets(ctx, 0, 2000, "foo", StatusVisible)
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) > 0 {
			t.Errorf("Got %d tweets, expected zero", len(out))
		}
	})

	t.Run("search", func(t *testing.T) {
		searchTerm := "oh"
		memDB := getPopulatedDB(t)
		memDB.EntriesPerPageMin = 1
		out, err := memDB.SearchTweets(ctx, 1, 10, searchTerm, StatusVisible)
		if err != nil {
			t.Error(err.Error())
		}
		lastDT := out[0].DateTime.UnixNano()
		for i, tweet := range out {
			if !strings.Contains(tweet.Body, searchTerm) {
				t.Errorf("Tweet body doesn't contain '%s': %s", searchTerm, tweet.Body)
			}
			if i > 0 && lastDT <= tweet.DateTime.UnixNano() {
				t.Error("tweets out of order")
			}
			lastDT = tweet.DateTime.UnixNano()
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		memDB := getPopulatedDB(t)
		memDB.EntriesPerPageMin = 1
		searchTerm := "o"
		_, err := memDB.SearchTweets(ctx, 1, 10, searchTerm, StatusVisible)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}
