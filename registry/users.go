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
	"strings"
	"time"

	"golang.org/x/xerrors"
)

// User represents a single twtxt.txt feed.
// The URL must be unique, but the Nick doesn't.
type User struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Nick          string    `json:"nick"`
	DateTimeAdded time.Time `json:"datetime_added"`
	LastSync      time.Time `json:"last_sync"`
}

// GetUserByURL returns the user's entire row from the database.
func (d *DB) GetUserByURL(userURL string) (*User, error) {
	userURL = strings.TrimSpace(userURL)
	if userURL == "" {
		return nil, xerrors.New("empty user URL provided")
	}

	user := User{}
	dtRaw := int64(0)
	lsRaw := int64(0)

	stmt := "SELECT * FROM users WHERE url = ?"
	err := d.conn.QueryRow(stmt, userURL).Scan(&user.ID, &user.URL, &user.Nick, &dtRaw, &lsRaw)
	if err != nil {
		return nil, xerrors.Errorf("unable to query for user with URL %s: %w", userURL, err)
	}

	user.DateTimeAdded = time.Unix(0, dtRaw)
	user.LastSync = time.Unix(0, lsRaw)

	return &user, nil
}

// InsertUser adds a user to the database.
// The ID field of the provided *User is ignored.
func (d *DB) InsertUser(u *User) error {
	if u == nil || u.URL == "" || u.Nick == "" {
		return xerrors.New("incomplete user info supplied: missing URL and/or nickname")
	}
	if u.DateTimeAdded.IsZero() {
		u.DateTimeAdded = time.Now().UTC()
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return xerrors.Errorf("couldn't begin transaction to insert user: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec("INSERT INTO users (url, nick, dt_added, last_sync) VALUES(?,?,?, 0)", u.URL, u.Nick, u.DateTimeAdded.UnixNano())
	if err != nil {
		return xerrors.Errorf("when inserting user to DB: %w", err)
	}

	return tx.Commit()
}

// DeleteUser removes a user and their tweets. Returns the number of tweets deleted.
func (d *DB) DeleteUser(u *User) (int64, error) {
	if u == nil || u.ID == "" {
		return 0, xerrors.New("invalid user provided")
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return 0, xerrors.Errorf("when beginning tx to delete user %s: %w", u.URL, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	delTweetsStmt := "DELETE FROM tweets WHERE user_id = ?"
	res, err := tx.Exec(delTweetsStmt, u.ID)
	if err != nil && !xerrors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	delUserStmt := "DELETE FROM users WHERE id = ?"
	_, err = tx.Exec(delUserStmt, u.ID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, xerrors.Errorf("when committing tx to delete user %s: %w", u.URL, err)
	}

	tweetsRemoved, err := res.RowsAffected()
	if err != nil {
		log.Printf("When getting number of tweets deleted when removing user %s: %s", u.URL, err)
	}

	return tweetsRemoved, nil
}

// GetUsers gets a page's worth of users.
func (d *DB) GetUsers(page, perPage int) ([]User, error) {
	if perPage < d.EntriesPerPageMin {
		perPage = d.EntriesPerPageMin
	}
	if perPage > d.EntriesPerPageMax {
		perPage = d.EntriesPerPageMax
	}
	if page < 0 {
		page = 0
	}
	idFloor := page * perPage
	idCeil := idFloor + perPage

	userStmt := "SELECT * FROM users WHERE id > ? AND id < ? ORDER BY dt_added DESC"
	rows, err := d.conn.Query(userStmt, idFloor, idCeil)
	if err != nil {
		return nil, xerrors.Errorf("when querying for users %d - %d: %w", idFloor+1, idCeil+1, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	users := make([]User, 0)
	for rows.Next() {
		dt := int64(0)
		ls := int64(0)
		thisUser := User{}
		err := rows.Scan(&thisUser.ID, &thisUser.URL, &thisUser.Nick, &dt, &ls)
		if err != nil {
			log.Printf("when querying for users %d - %d: %s", idFloor+1, idCeil+1, err)
			continue
		}
		thisUser.DateTimeAdded = time.Unix(0, dt)
		thisUser.LastSync = time.Unix(0, ls)
		users = append(users, thisUser)
	}

	return users, nil
}
