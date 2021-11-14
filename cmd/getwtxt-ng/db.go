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

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/xerrors"
)

func initDB(dbPath string) (*sql.DB, error) {
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
		db.Close()
		return nil, xerrors.Errorf("while creating users table at %s :: %w", dbPath, err)
	}

	createTweetsTableStr := `CREATE TABLE IF NOT EXISTS tweets (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	user_id INTEGER NOT NULL,
    	dt INTEGER NOT NULL,
    	body TEXT NOT NULL,
    	body_hash TEXT NOT NULL UNIQUE
)`
	_, err = db.Exec(createTweetsTableStr)
	if err != nil {
		db.Close()
		return nil, xerrors.Errorf("while creating tweets table at %s :: %w", dbPath, err)
	}

	return db, nil
}