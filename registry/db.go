// Package registry implements a SQLite3 twtxt registry back-end.
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
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

// DB contains the database connection pool and associated settings.
type DB struct {
	// EntriesPerPageMin specifies the minimum number of users or tweets to display in a single page.
	EntriesPerPageMin int

	// EntriesPerPageMax specifies the maximum number of users or tweets to display in a single page.
	EntriesPerPageMax int

	// Client is the default HTTP client, which has a 5-second timeout.
	Client *http.Client

	logger *log.Logger
	conn   *sql.DB
}

// InitSQLite initializes the registry's database, creating the appropriate tables if needed.
func InitSQLite(dbPath string, maxEntriesPerPage, minEntriesPerPage int, httpClient *http.Client, logger *log.Logger) (*DB, error) {
	shouldInit := dbPath == ":memory:"
	if !shouldInit {
		_, err := os.Stat(dbPath)
		if errors.Is(err, fs.ErrNotExist) {
			shouldInit = true
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("while initializing connection to sqlite3 db at %s :: %w", dbPath, err)
	}

	if shouldInit {
		createUserTableStr := `CREATE TABLE IF NOT EXISTS users (
    		id INTEGER PRIMARY KEY AUTOINCREMENT,
    		url TEXT NOT NULL UNIQUE,
    		nick TEXT NOT NULL,
    		passcode_hash BLOB NOT NULL,
    		dt_added INTEGER NOT NULL,
    		last_sync INTEGER NOT NULL
		)`
		_, err = db.Exec(createUserTableStr)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating users table at %s :: %w", dbPath, err)
		}

		createTweetsTableStr := `CREATE TABLE IF NOT EXISTS tweets (
    		id INTEGER PRIMARY KEY AUTOINCREMENT,
    		user_id INTEGER NOT NULL,
    		dt INTEGER NOT NULL,
    		body TEXT NOT NULL,
    		contains_mentions INTEGER NOT NULL DEFAULT 0,
    		contains_tags INTEGER NOT NULL DEFAULT 0,
    		hidden INTEGER NOT NULL DEFAULT 0,
    		UNIQUE (user_id, dt, body) ON CONFLICT IGNORE,
    		FOREIGN KEY(user_id) REFERENCES users(id)
		)`
		_, err = db.Exec(createTweetsTableStr)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating tweets table at %s :: %w", dbPath, err)
		}

		createTweetsViewStr := `CREATE VIEW tweets_users (
    		id, user_id, nick, url, dt, body, contains_mentions, contains_tags, hidden		
		) AS 
    		SELECT 
    		    tweets.id, tweets.user_id, users.nick, users.url, tweets.dt, tweets.body,
    		    tweets.contains_mentions, tweets.contains_tags, tweets.hidden
    		FROM tweets
    		JOIN users ON users.id = tweets.user_id`
		_, err = db.Exec(createTweetsViewStr)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating tweets/users view at %s :: %w", dbPath, err)
		}

		createTweetSearchTableStr := `CREATE VIRTUAL TABLE tweets_search USING fts5 (
    		id, user_id, nick, url, dt, body, contains_mentions, contains_tags, hidden,
    		content = tweets_users,
    		content_rowid = id,
    		columnsize = 0,
		)`
		_, err = db.Exec(createTweetSearchTableStr)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating tweets_search virtual table :: %s", err)
		}

		createTweetSearchTableTriggerInsert := `CREATE TRIGGER tweetsInsert AFTER INSERT ON tweets
    		BEGIN
				INSERT INTO tweets_search (
					ROWID, user_id, dt, body, contains_mentions, contains_tags, hidden
				) SELECT ROWID, user_id, dt, body, contains_mentions, contains_tags, hidden FROM tweets WHERE ROWID = NEW.ROWID;
			END;`
		_, err = db.Exec(createTweetSearchTableTriggerInsert)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating tweets_search virtual table insert trigger :: %s", err)
		}

		createTweetSearchTableTriggerDelete := `CREATE TRIGGER tweetsDelete AFTER DELETE ON tweets
    		BEGIN
    		    DELETE FROM tweets_search WHERE ROWID = OLD.ROWID;
			END;`
		_, err = db.Exec(createTweetSearchTableTriggerDelete)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("while creating tweets_search virtual table delete trigger :: %s", err)
		}
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	dbWrap := DB{
		conn:              db,
		logger:            logger,
		EntriesPerPageMin: minEntriesPerPage,
		EntriesPerPageMax: maxEntriesPerPage,
		Client:            httpClient,
	}

	return &dbWrap, nil
}
