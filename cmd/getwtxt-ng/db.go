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
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/xerrors"
)

type User struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Nick          string    `json:"nick"`
	DateTimeAdded time.Time `json:"datetime_added"`
}

type Tweet struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	DateTime time.Time `json:"datetime"`
	Body     string    `json:"body"`
}

type DB struct {
	*sql.DB
}

func initDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, xerrors.Errorf("while initializing connection to sqlite3 db at %s :: %w", dbPath, err)
	}

	createUserTableStr := `CREATE TABLE IF NOT EXISTS users (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	url TEXT NOT NULL UNIQUE,
    	nick TEXT NOT NULL,
    	dt_added INTEGER NOT NULL
)`
	_, err = db.Exec(createUserTableStr)
	if err != nil {
		_ = db.Close()
		return nil, xerrors.Errorf("while creating users table at %s :: %w", dbPath, err)
	}

	createTweetsTableStr := `CREATE TABLE IF NOT EXISTS tweets (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	user_id INTEGER NOT NULL,
    	dt INTEGER NOT NULL,
    	body TEXT NOT NULL,
    	body_hash TEXT NOT NULL,
    	UNIQUE (user_id, dt, body) ON CONFLICT IGNORE
)`
	_, err = db.Exec(createTweetsTableStr)
	if err != nil {
		_ = db.Close()
		return nil, xerrors.Errorf("while creating tweets table at %s :: %w", dbPath, err)
	}

	return &DB{db}, nil
}

func (d *DB) InsertUser(u *User) error {
	if u == nil || u.URL == "" || u.Nick == "" {
		return xerrors.New("incomplete user info supplied: missing URL and/or nickname")
	}
	tx, err := d.Begin()
	if err != nil {
		return xerrors.Errorf("couldn't begin transaction to insert user: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO users (url, nick, dt_added) VALUES(?,?,?)", u.URL, u.Nick, time.Now().UTC().Unix()); err != nil {
		return xerrors.Errorf("when inserting user to DB: %w", err)
	}
	return tx.Commit()
}
