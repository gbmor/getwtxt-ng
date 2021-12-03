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
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/xerrors"
)

type User struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Nick          string    `json:"nick"`
	DateTimeAdded time.Time `json:"datetime_added"`
	LastSync      time.Time `json:"last_sync"`
}

type Tweet struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	DateTime time.Time `json:"datetime"`
	Body     string    `json:"body"`
	Hidden   int       `json:"hidden"`
}

type DB struct {
	conn *sql.DB
}

type TweetVisibilityStatus int

const (
	StatusVisible TweetVisibilityStatus = iota
	StatusHidden
)

// InitDB initializes the registry's database, creating the appropriate tables if needed.
func InitDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, xerrors.Errorf("while initializing connection to sqlite3 db at %s :: %w", dbPath, err)
	}

	createUserTableStr := `CREATE TABLE IF NOT EXISTS users (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	url TEXT NOT NULL UNIQUE,
    	nick TEXT NOT NULL,
    	dt_added INTEGER NOT NULL,
    	last_sync INTEGER NOT NULL
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
    	hidden INTEGER NOT NULL DEFAULT 0,
    	UNIQUE (user_id, dt, body) ON CONFLICT IGNORE,
    	FOREIGN KEY(user_id) REFERENCES users(id)
)`
	_, err = db.Exec(createTweetsTableStr)
	if err != nil {
		_ = db.Close()
		return nil, xerrors.Errorf("while creating tweets table at %s :: %w", dbPath, err)
	}

	return &DB{conn: db}, nil
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

	user.DateTimeAdded = time.Unix(dtRaw, 0)
	user.LastSync = time.Unix(lsRaw, 0)

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

	_, err = tx.Exec("INSERT INTO users (url, nick, dt_added, last_sync) VALUES(?,?,?, 0)", u.URL, u.Nick, u.DateTimeAdded.Unix())
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

// InsertTweets adds a collection of tweets to the database.
func (d *DB) InsertTweets(tweets []Tweet) error {
	if len(tweets) == 0 {
		return xerrors.New("invalid tweets provided")
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return xerrors.Errorf("when beginning tx to insert tweets: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	insertStmt := "INSERT INTO tweets (user_id, dt, body) VALUES(?,?,?)"
	stmt, err := tx.Prepare(insertStmt)
	if err != nil {
		return xerrors.Errorf("could not prepare statement to insert tweets: %w", err)
	}

	for _, t := range tweets {
		if _, err := stmt.Exec(t.UserID, t.DateTime.Unix(), t.Body); err != nil {
			return xerrors.Errorf("could not insert tweet for uid %s at %s: %w", t.UserID, t.DateTime, err)
		}
	}

	return tx.Commit()
}

// ToggleTweetHiddenStatus changes the provided tweet's hidden status.
func (d *DB) ToggleTweetHiddenStatus(userID string, timestamp time.Time, status TweetVisibilityStatus) error {
	if userID == "" || timestamp.IsZero() {
		return xerrors.New("invalid user ID or tweet timestamp provided")
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return xerrors.Errorf("when beginning tx to hide tweet by %s at %s: %w", userID, timestamp, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	toggleStmt := "UPDATE tweets SET hidden = ? WHERE user_id = ? AND dt = ?"
	if _, err := tx.Exec(toggleStmt, status, userID, timestamp.Unix()); err != nil {
		return xerrors.Errorf("error hiding tweet by %s at %s: %w", userID, timestamp, err)
	}

	return tx.Commit()
}

// GetTweets gets a page's worth of tweets.
func (d *DB) GetTweets(page, perPage int) ([]Tweet, error) {
	if perPage < 20 {
		perPage = 20
	}
	if perPage > 1000 {
		perPage = 1000
	}
	if page < 0 {
		page = 0
	}
	idFloor := page * perPage
	idCeil := idFloor + perPage

	tweetStmt := "SELECT * FROM tweets WHERE id > ? AND id < ? ORDER BY dt DESC"
	rows, err := d.conn.Query(tweetStmt, idFloor, idCeil)
	if err != nil {
		return nil, xerrors.Errorf("when querying for tweets %d - %d: %w", idFloor+1, idCeil+1, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tweets := make([]Tweet, 0)
	for rows.Next() {
		dt := int64(0)
		thisTweet := Tweet{}
		err := rows.Scan(&thisTweet.ID, &thisTweet.UserID, &dt, &thisTweet.Body, &thisTweet.Hidden)
		if err != nil {
			log.Printf("when querying for tweets %d - %d: %s", idFloor+1, idCeil+1, err)
			continue
		}
		thisTweet.DateTime = time.Unix(dt, 0)
		tweets = append(tweets, thisTweet)
	}

	return tweets, nil
}
