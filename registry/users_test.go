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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gbmor/getwtxt-ng/common"
)

func TestDB_GetUserByURL(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	memDB := getPopulatedDB(t)
	ctx := context.Background()
	defer func() {
		if err := memDB.conn.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	t.Run("invalid user URL", func(t *testing.T) {
		db := DB{}
		_, err := db.GetUserByURL(ctx, "    ")
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
		_, err := mockDB.GetUserByURL(ctx, "https://example.net/twtxt.txt")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("get a user successfully", func(t *testing.T) {
		out, err := memDB.GetUserByURL(ctx, "https://example.com/twtxt.txt")
		if err != nil {
			t.Error(err.Error())
		}
		if out.Nick != "foobar" {
			t.Errorf("Expected nick 'foobar', got '%s'", out.Nick)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := memDB.GetUserByURL(ctx, "https://example.com/twtxt.txt")
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_InsertUser(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	memDB := getPopulatedDB(t)
	ctx := context.Background()

	passcodeHash, err := common.HashPass("abcdefghij0123456789")
	if err != nil {
		t.Error(err.Error())
		return
	}
	testUser := User{
		URL:          "https://example.net/twtxt.txt",
		Nick:         "foobaz",
		PasscodeHash: passcodeHash,
	}
	insertStmt := "INSERT INTO users (url, nick, passcode_hash, dt_added, last_sync) VALUES(?,?,?,?, 0)"

	t.Run("invalid params provided", func(t *testing.T) {
		db := DB{}
		err := db.InsertUser(ctx, nil)
		if err == nil {
			t.Error("Expected error, but got nil")
		}
		if !strings.Contains(err.Error(), "incomplete user info") {
			t.Errorf("Expected incomplete user info error, got: %s", err)
		}
	})

	t.Run("error beginning tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		err := mockDB.InsertUser(ctx, &testUser)
		if !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to insert user, tx done", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(insertStmt).
			WithArgs(testUser.URL, testUser.Nick, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		err := mockDB.InsertUser(ctx, &testUser)
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("fail to insert user, dupe", func(t *testing.T) {
		err := memDB.InsertUser(ctx, &populatedDBUsers[0])
		if err == nil {
			t.Error("Expected error inserting duplicate user, got nil")
		}
	})

	t.Run("insert new user", func(t *testing.T) {
		err := memDB.InsertUser(ctx, &testUser)
		if err != nil {
			t.Error(err.Error())
		}
		getUser := "SELECT * FROM users WHERE url = ?"
		dbUser := User{}
		dt := int64(0)
		err = memDB.conn.QueryRow(getUser, testUser.URL).Scan(&dbUser.ID, &dbUser.URL, &dbUser.Nick, &dbUser.PasscodeHash, &dt, &dt)
		if err != nil {
			t.Error(err.Error())
		}
		testUser.DateTimeAdded = time.Time{}
		testUser.LastSync = time.Time{}
		testUser.ID = dbUser.ID
		checkedUser := testUser
		checkedUser.PasscodeHash = ""
		dbUser.PasscodeHash = ""
		if !reflect.DeepEqual(checkedUser, dbUser) {
			t.Errorf("Expected:\n%#v\nGot:\n%#v\n", checkedUser, dbUser)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := memDB.InsertUser(ctx, &testUser)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_DeleteUser(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	delTweetsStmt := "DELETE FROM tweets WHERE user_id = ?"
	delUserStmt := "DELETE FROM users WHERE id = ?"

	t.Run("invalid user info", func(t *testing.T) {
		emptyUser := User{}
		_, err := mockDB.DeleteUser(ctx, &emptyUser)
		if !strings.Contains(err.Error(), "invalid user provided") {
			t.Errorf("Expected invalid user error, got: %s", err)
		}
	})

	t.Run("fail to begin tx", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
		_, err := mockDB.DeleteUser(ctx, &populatedDBUsers[0])
		if !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("Expected sql.ErrConnDone, got: %s", err)
		}
	})

	t.Run("fail to delete tweets", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(delTweetsStmt).
			WithArgs(populatedDBUsers[0].ID).
			WillReturnError(sql.ErrTxDone)
		mock.ExpectRollback()
		_, err := mockDB.DeleteUser(ctx, &populatedDBUsers[0])
		if !errors.Is(err, sql.ErrTxDone) {
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
		_, err := mockDB.DeleteUser(ctx, &populatedDBUsers[0])
		if !errors.Is(err, sql.ErrTxDone) {
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
		_, err := mockDB.DeleteUser(ctx, &populatedDBUsers[0])
		if !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Expected sql.ErrTxDone, got: %s", err)
		}
	})

	t.Run("successful", func(t *testing.T) {
		tweets, err := memDB.DeleteUser(ctx, &populatedDBUsers[0])
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
			if !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("Expected row to be missing? %s", err)
			}
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := memDB.DeleteUser(ctx, &populatedDBUsers[0])
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}

func TestDB_GetUsers(t *testing.T) {
	memDB := getPopulatedDB(t)
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	userStmt := `SELECT id, url, nick, dt_added, last_sync
					FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY dt_added DESC) AS set_id FROM users)
					WHERE set_id > ?
  					AND set_id <= ?`

	t.Run("error on query", func(t *testing.T) {
		mock.ExpectQuery(userStmt).
			WithArgs(0, 20).
			WillReturnError(sql.ErrNoRows)
		_, err := mockDB.GetUsers(ctx, -1, 2)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("fail to scan", func(t *testing.T) {
		mock.ExpectQuery(userStmt).
			WithArgs(0, 1000).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "url", "nick", "dt_added", "last_sync"}).
					AddRow("1", "https://example.com", "foobar", "thirty five o'clock", "sync time"))
		out, err := mockDB.GetUsers(ctx, 0, 2000)
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) > 0 {
			t.Errorf("Got %d users, expected zero", len(out))
		}
	})

	t.Run("get users", func(t *testing.T) {
		out, err := memDB.GetUsers(ctx, 0, 20)
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) != len(populatedDBUsers) {
			t.Errorf("Expected %d tweets, got %d", len(populatedDBUsers), len(out))
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := memDB.GetUsers(ctx, 0, 20)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestDB_SearchUsers(t *testing.T) {
	mockDB, mock := getDBMocker(t)
	ctx := context.Background()
	searchTerm := "%foo%"
	searchStmt := `SELECT id, url, nick, dt_added, last_sync
					FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY dt_added DESC) AS set_id FROM users WHERE nick LIKE ? OR url LIKE ?)
					WHERE set_id > ?
  					AND set_id <= ?`

	t.Run("error on query", func(t *testing.T) {
		mock.ExpectQuery(searchStmt).
			WithArgs(searchTerm, searchTerm, 0, 20).
			WillReturnError(sql.ErrNoRows)
		_, err := mockDB.SearchUsers(ctx, 1, 3, "foo")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows, got: %s", err)
		}
	})

	t.Run("fail to scan", func(t *testing.T) {
		mock.ExpectQuery(searchStmt).
			WithArgs(searchTerm, searchTerm, 0, 1000).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "nick", "dt_added", "last_sync"}).
				AddRow(5, "https://example.com/twtxt.txt", "foo", "eleventy-three o'clock", 0))
		out, err := mockDB.SearchUsers(ctx, 0, 5000, "foo")
		if err != nil {
			t.Error(err.Error())
		}
		if len(out) > 0 {
			t.Errorf("Expected 0 users, got %d", len(out))
		}
	})

	t.Run("search", func(t *testing.T) {
		searchTerm := "example"
		memDB := getPopulatedDB(t)
		out, err := memDB.SearchUsers(ctx, 1, 20, searchTerm)
		if err != nil {
			t.Error(err.Error())
		}
		lastDT := out[0].DateTimeAdded.UnixNano()
		for i, user := range out {
			if !strings.Contains(user.URL, searchTerm) && !strings.Contains(user.Nick, searchTerm) {
				t.Errorf("User nick and URL don't contain '%s': %s %s", searchTerm, user.Nick, user.URL)
			}
			if i > 0 && lastDT <= user.DateTimeAdded.UnixNano() {
				t.Error("tweets out of order")
			}
			lastDT = user.DateTimeAdded.UnixNano()
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		memDB := getPopulatedDB(t)
		_, err := memDB.SearchUsers(ctx, 1, 20, searchTerm)
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err.Error())
	}
}
