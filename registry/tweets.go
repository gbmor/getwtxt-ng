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
	"errors"
	"fmt"
	"strings"
	"time"
)

// Tweet represents a single entry in a User's twtxt.txt file.
// Uniqueness must be preserved over (UserID, DateTime, Body).
type Tweet struct {
	ID       string                `json:"id"`
	UserID   string                `json:"user_id"`
	Nickname string                `json:"nickname"`
	URL      string                `json:"url"`
	DateTime time.Time             `json:"datetime"`
	Body     string                `json:"body"`
	Hidden   TweetVisibilityStatus `json:"hidden"`
}

type TweetVisibilityStatus int

const (
	StatusVisible TweetVisibilityStatus = iota
	StatusHidden
)

func FormatTweetsPlain(tweets []Tweet) string {
	if len(tweets) < 1 {
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(len(tweets) * 256)
	for _, tweet := range tweets {
		builder.WriteString(tweet.Nickname)
		builder.WriteString("\t")
		builder.WriteString(tweet.URL)
		builder.WriteString("\t")
		builder.WriteString(tweet.DateTime.Format(time.RFC3339))
		builder.WriteString("\t")
		builder.WriteString(tweet.Body)
		builder.WriteString("\n")
	}

	return builder.String()
}

// InsertTweets adds a collection of tweets to the database.
func (d *DB) InsertTweets(ctx context.Context, tweets []Tweet) error {
	if len(tweets) == 0 {
		return errors.New("invalid tweets provided")
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("when beginning tx to insert tweets: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	insertStmt := "INSERT OR IGNORE INTO tweets (user_id, dt, body) VALUES(?,?,?)"
	stmt, err := tx.Prepare(insertStmt)
	if err != nil {
		return fmt.Errorf("could not prepare statement to insert tweets: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, t := range tweets {
		if _, err := stmt.ExecContext(ctx, t.UserID, t.DateTime.UnixNano(), t.Body); err != nil {
			return fmt.Errorf("could not insert tweet for uid %s at %s: %w", t.UserID, t.DateTime, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing tx to insert tweets: %w", err)
	}

	return nil
}

// ToggleTweetHiddenStatus changes the provided tweet's hidden status.
func (d *DB) ToggleTweetHiddenStatus(ctx context.Context, userID string, timestamp time.Time, status TweetVisibilityStatus) error {
	if userID == "" || timestamp.IsZero() {
		return errors.New("invalid user ID or tweet timestamp provided")
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("when beginning tx to hide tweet by %s at %s: %w", userID, timestamp, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	toggleStmt := "UPDATE tweets SET hidden = ? WHERE user_id = ? AND dt = ?"
	if _, err := tx.ExecContext(ctx, toggleStmt, status, userID, timestamp.UnixNano()); err != nil {
		return fmt.Errorf("error hiding tweet by %s at %s: %w", userID, timestamp, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing tx to set hidden status of tweet by user %s at %s to %d: %w", userID, timestamp, status, err)
	}

	return nil
}

// GetTweets retrieves a page's worth of tweets in descending order by datetime.
func (d *DB) GetTweets(ctx context.Context, page, perPage int, visibilityStatus TweetVisibilityStatus) ([]Tweet, error) {
	page--
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

	tweetStmt := `SELECT id, user_id, nick, url, dt, body, hidden
					FROM (SELECT tweets.*, users.nick AS nick, users.url AS url, ROW_NUMBER() OVER (ORDER BY dt DESC) AS set_id
					      FROM tweets LEFT JOIN users ON users.id = tweets.user_id WHERE tweets.hidden = ?)
					WHERE set_id > ?
  					AND set_id <= ?`
	rows, err := d.conn.QueryContext(ctx, tweetStmt, visibilityStatus, idFloor, idCeil)
	if err != nil {
		return nil, fmt.Errorf("when querying for tweets %d - %d: %w", idFloor+1, idCeil+1, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tweets := make([]Tweet, 0)
	for rows.Next() {
		dt := int64(0)
		thisTweet := Tweet{}
		err := rows.Scan(&thisTweet.ID, &thisTweet.UserID, &thisTweet.Nickname, &thisTweet.URL, &dt, &thisTweet.Body, &thisTweet.Hidden)
		if err != nil {
			d.logger.Debugf("when querying for tweets %d - %d: %s", idFloor+1, idCeil+1, err)
			continue
		}
		thisTweet.DateTime = time.Unix(0, dt)
		tweets = append(tweets, thisTweet)
	}

	return tweets, nil
}

// SearchTweets searches for a given term in tweet bodies and returns a page worth in descending order by datetime.
func (d *DB) SearchTweets(ctx context.Context, page, perPage int, searchTerm string, visibilityStatus TweetVisibilityStatus) ([]Tweet, error) {
	// SQLite expects the format %term% for arbitrary characters on either side of the search term.
	searchTerm = fmt.Sprintf("%%%s%%", searchTerm)
	page--
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

	searchStmt := `SELECT id, user_id, nick, url, dt, body, hidden
					FROM (SELECT tweets.*, users.nick AS nick, users.url AS url, ROW_NUMBER() OVER (ORDER BY dt DESC) AS set_id
					      FROM tweets LEFT JOIN users ON users.id = tweets.user_id WHERE tweets.hidden = ? AND body LIKE ?)
					WHERE set_id > ?
  					AND set_id <= ?`
	rows, err := d.conn.QueryContext(ctx, searchStmt, visibilityStatus, searchTerm, idFloor, idCeil)
	if err != nil {
		return nil, fmt.Errorf("when querying for tweets containing %s, %d - %d: %w", searchTerm, idFloor+1, idCeil, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tweets := make([]Tweet, 0)
	for rows.Next() {
		dt := int64(0)
		thisTweet := Tweet{}
		err := rows.Scan(&thisTweet.ID, &thisTweet.UserID, &thisTweet.Nickname, &thisTweet.URL, &dt, &thisTweet.Body, &thisTweet.Hidden)
		if err != nil {
			d.logger.Debugf("when querying for tweets containing %s, %d - %d: %s", searchTerm, idFloor+1, idCeil+1, err)
			continue
		}
		thisTweet.DateTime = time.Unix(0, dt)
		tweets = append(tweets, thisTweet)
	}

	return tweets, nil
}
