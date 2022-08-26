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
	"fmt"
	"net/http"
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
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("while initializing connection to sqlite3 db at %s :: %w", dbPath, err)
	}

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
    	hidden INTEGER NOT NULL DEFAULT 0,
    	UNIQUE (user_id, dt, body) ON CONFLICT IGNORE,
    	FOREIGN KEY(user_id) REFERENCES users(id)
)`
	_, err = db.Exec(createTweetsTableStr)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("while creating tweets table at %s :: %w", dbPath, err)
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
