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
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gbmor/getwtxt-ng/common"
)

// ErrNoUsersProvided is returned when user(s) were expected and not provided.
var ErrNoUsersProvided = errors.New("no user(s) provided")

// ErrIncompleteUserInfo is returned when we need more information than we were given.
var ErrIncompleteUserInfo = errors.New("incomplete user info supplied: missing URL and/or nickname and/or passcode")

// ErrUserURLIsNotTwtxtFile is returned when the provided user's URL is not a path to a twtxt.txt file.
var ErrUserURLIsNotTwtxtFile = errors.New("user URL does not point to twtxt.txt")

// RegexIsAlpha matches `[a-zA-Z0-9_]+`
var RegexIsAlpha = regexp.MustCompile(`\w+`)

// RegexURLIsTwtxtFile checks if the URL points to a twtxt.txt file.
var RegexURLIsTwtxtFile = regexp.MustCompile(`/twtxt\.txt$`)

// User represents a single twtxt.txt feed.
// The URL must be unique, but the Nick doesn't.
type User struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Nick          string    `json:"nickname"`
	Passcode      string    `json:"-"`
	PasscodeHash  []byte    `json:"-"`
	DateTimeAdded time.Time `json:"datetime_added"`
	LastSync      time.Time `json:"last_sync"`
}

func FormatUsersPlain(users []User) string {
	if len(users) < 1 {
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(len(users) * 128)
	for _, user := range users {
		builder.WriteString(user.Nick)
		builder.WriteString("\t")
		builder.WriteString(user.URL)
		builder.WriteString("\t")
		builder.WriteString(user.DateTimeAdded.Format(time.RFC3339))
		builder.WriteString("\t")
		builder.WriteString(user.LastSync.Format(time.RFC3339))
		builder.WriteString("\n")
	}

	return builder.String()
}

// GeneratePasscode creates a new passcode for a user, then stores it and its bcrypt hash in the User struct.
// The plaintext passcode is returned on success.
// Both the ciphertext and the plaintext passcode will be omitted if you serialize the User struct into JSON.
func (u *User) GeneratePasscode() (string, error) {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("couldn't generate random bytes for user's passcode: %w", err)
	}

	u.Passcode = fmt.Sprintf("%x", b)
	u.PasscodeHash, err = common.HashPass(u.Passcode)
	if err != nil {
		return "", fmt.Errorf("couldn't get hash of user's passcode: %w", err)
	}

	return u.Passcode, nil
}

// GetFullUserByURL returns the user's entire row from the database.
func (d *DB) GetFullUserByURL(ctx context.Context, userURL string) (*User, error) {
	userURL = strings.TrimSpace(userURL)
	if userURL == "" {
		return nil, ErrNoUsersProvided
	}

	user := User{}
	dtRaw := int64(0)
	lsRaw := int64(0)

	stmt := "SELECT * FROM users WHERE url = ?"
	err := d.conn.QueryRowContext(ctx, stmt, userURL).Scan(&user.ID, &user.URL, &user.Nick, &user.PasscodeHash, &dtRaw, &lsRaw)
	if err != nil {
		return nil, fmt.Errorf("unable to query for user with URL %s: %w", userURL, err)
	}

	user.DateTimeAdded = time.Unix(0, dtRaw)
	user.LastSync = time.Unix(0, lsRaw)

	return &user, nil
}

// InsertUser adds a user to the database.
// The ID field of the provided *User is ignored.
func (d *DB) InsertUser(ctx context.Context, u *User) error {
	if u == nil || u.URL == "" || u.Nick == "" || len(u.PasscodeHash) < 1 ||
		!RegexIsAlpha.MatchString(u.Nick) {
		return ErrIncompleteUserInfo
	}
	parsedURL, urlParseErr := url.Parse(u.URL)
	if urlParseErr != nil || parsedURL.Scheme == "" {
		return ErrIncompleteUserInfo
	}

	if !RegexURLIsTwtxtFile.MatchString(u.URL) {
		return ErrUserURLIsNotTwtxtFile
	}

	if u.DateTimeAdded.IsZero() {
		u.DateTimeAdded = time.Now().UTC()
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("couldn't begin transaction to insert user: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, "INSERT INTO users (url, nick, passcode_hash, dt_added, last_sync) VALUES(?,?,?,?, 0)",
		u.URL, u.Nick, u.PasscodeHash, u.DateTimeAdded.UnixNano())
	if err != nil {
		return fmt.Errorf("when inserting user to DB: %w", err)
	}

	userID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not retrieve new user's ID")
	}

	u.ID = fmt.Sprintf("%d", userID)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing tx to insert user %s %s: %w", u.Nick, u.URL, err)
	}

	return nil
}

// DeleteUser removes a user and their tweets. Returns the number of tweets deleted.
func (d *DB) DeleteUser(ctx context.Context, u *User) (int64, error) {
	if u == nil || u.ID == "" {
		return 0, ErrNoUsersProvided
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("when beginning tx to delete user %s: %w", u.URL, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	delTweetsStmt := "DELETE FROM tweets WHERE user_id = ?"
	res, err := tx.ExecContext(ctx, delTweetsStmt, u.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("could not delete tweets for user %s: %w", u.ID, err)
	}

	delUserStmt := "DELETE FROM users WHERE id = ?"
	_, err = tx.ExecContext(ctx, delUserStmt, u.ID)
	if err != nil {
		return 0, fmt.Errorf("could not delete user %s: %w", u.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("when committing tx to delete user %s: %w", u.URL, err)
	}

	tweetsRemoved, err := res.RowsAffected()
	if err != nil {
		d.logger.Debugf("When getting number of tweets deleted when removing user %s: %s", u.URL, err)
	}

	return tweetsRemoved, nil
}

// DeleteUsers removes multiple users and their tweets. Returns the total number of tweets deleted.
func (d *DB) DeleteUsers(ctx context.Context, urls []string) (int64, error) {
	userCount := len(urls)
	if userCount < 1 {
		return 0, ErrNoUsersProvided
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("when beginning tx to delete %d users: %w", userCount, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	tweetCount := int64(0)
	delTweetsStmtStr := "DELETE FROM tweets WHERE user_id IN (SELECT id FROM users WHERE url = ?)"
	delTweetsStmt, err := tx.Prepare(delTweetsStmtStr)
	if err != nil {
		return 0, fmt.Errorf("when preparing stmt to delete tweets from %d users: %w", userCount, err)
	}

	delUserStmtStr := "DELETE FROM users WHERE url = ?"
	delUserStmt, err := tx.Prepare(delUserStmtStr)
	if err != nil {
		return 0, fmt.Errorf("when preparing stmt to delete %d users: %w", userCount, err)
	}

	for _, user := range urls {
		tweetRes, err := delTweetsStmt.ExecContext(ctx, user)
		if err != nil {
			return 0, fmt.Errorf("when deleting tweets for user %s: %w", user, err)
		}
		thisTweetCount, err := tweetRes.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("when deleting tweets for user %s: %w", user, err)
		}
		tweetCount += thisTweetCount

		_, err = delUserStmt.ExecContext(ctx, user)
		if err != nil {
			return 0, fmt.Errorf("when deleting user %s: %w", user, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("when committing tx to delete %d users: %w", userCount, err)
	}

	return tweetCount, nil
}

// GetUsers gets a page's worth of users.
func (d *DB) GetUsers(ctx context.Context, page, perPage int) ([]User, error) {
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

	userStmt := `SELECT id, url, nick, dt_added, last_sync
					FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY dt_added DESC) AS set_id FROM users)
					WHERE set_id > ?
  					AND set_id <= ?`
	rows, err := d.conn.QueryContext(ctx, userStmt, idFloor, idCeil)
	if err != nil {
		return nil, fmt.Errorf("when querying for users %d - %d: %w", idFloor+1, idCeil+1, err)
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
			d.logger.Debugf("when querying for users %d - %d: %s", idFloor+1, idCeil+1, err)
			continue
		}
		thisUser.DateTimeAdded = time.Unix(0, dt)
		thisUser.LastSync = time.Unix(0, ls)
		users = append(users, thisUser)
	}

	return users, nil
}

// SearchUsers returns a paginated list of users whose nicknames or URLs match the query.
func (d *DB) SearchUsers(ctx context.Context, page, perPage int, searchTerm string) ([]User, error) {
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

	searchStmt := `SELECT id, url, nick, dt_added, last_sync
					FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY dt_added DESC) AS set_id FROM users WHERE nick LIKE ? OR url LIKE ?)
					WHERE set_id > ?
  					AND set_id <= ?`
	rows, err := d.conn.QueryContext(ctx, searchStmt, searchTerm, searchTerm, idFloor, idCeil)
	if err != nil {
		return nil, fmt.Errorf("when querying for users containing %s, %d - %d: %w", searchTerm, idFloor+1, idCeil, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	users := make([]User, 0)
	for rows.Next() {
		dt := int64(0)
		dtSync := int64(0)
		thisUser := User{}
		err := rows.Scan(&thisUser.ID, &thisUser.URL, &thisUser.Nick, &dt, &dtSync)
		if err != nil {
			d.logger.Debugf("when querying for users containing %s, %d - %d: %s", searchTerm, idFloor+1, idCeil+1, err)
			continue
		}
		thisUser.DateTimeAdded = time.Unix(0, dt)
		thisUser.LastSync = time.Unix(0, dtSync)
		users = append(users, thisUser)
	}

	return users, nil
}
