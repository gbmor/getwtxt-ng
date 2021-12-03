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
	"log"
	"time"

	"golang.org/x/xerrors"
)

// Tweet represents a single entry in a User's twtxt.txt file.
// Uniqueness must be preserved over (UserID, DateTime, Body).
type Tweet struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	DateTime time.Time `json:"datetime"`
	Body     string    `json:"body"`
	Hidden   int       `json:"hidden"`
}

type TweetVisibilityStatus int

const (
	StatusVisible TweetVisibilityStatus = iota
	StatusHidden
)

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
		if _, err := stmt.Exec(t.UserID, t.DateTime.UnixNano(), t.Body); err != nil {
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
	if _, err := tx.Exec(toggleStmt, status, userID, timestamp.UnixNano()); err != nil {
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
		thisTweet.DateTime = time.Unix(0, dt)
		tweets = append(tweets, thisTweet)
	}

	return tweets, nil
}
