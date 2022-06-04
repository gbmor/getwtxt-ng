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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gbmor/getwtxt-ng/common"
)

// FetchTwtxt grabs the twtxt file from the provided URL.
// The If-Modified-Since header is set to the time provided.
// Comments and whitespace are stripped from the response.
// If we receive a 304, return a nil slice and a nil error.
func (d *DB) FetchTwtxt(twtxtURL, userID string, lastModified time.Time) ([]Tweet, error) {
	if !common.IsValidURL(twtxtURL, d.logger) {
		return nil, fmt.Errorf("invalid URL provided: %s", twtxtURL)
	}
	if d == nil || d.Client == nil {
		return nil, fmt.Errorf("can't fetch twtxt file at %s: have nil receiver or nil HTTP client", twtxtURL)
	}

	req, err := http.NewRequest("GET", twtxtURL, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create http request to fetch %s: %w", twtxtURL, err)
	}
	req.Header.Set("If-Modified-Since", lastModified.Format(time.RFC1123))

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making http request to %s: %w", twtxtURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got status code %d from %s", resp.StatusCode, twtxtURL)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		return nil, fmt.Errorf("received non-text/plain content type from %s: %s", twtxtURL, contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body from %s: %w", twtxtURL, err)
	}

	bodySplit := strings.Split(string(body), "\n")
	tweets := make([]Tweet, 0, 256)

	for _, e := range bodySplit {
		e = strings.TrimSpace(e)
		if strings.HasPrefix(e, "#") || e == "" {
			continue
		}

		tweetHalves := strings.Fields(e)
		thisTweet := Tweet{
			UserID: userID,
			Body:   tweetHalves[1],
		}

		if strings.Contains(tweetHalves[0], ".") {
			thisTweet.DateTime, err = time.Parse(time.RFC3339Nano, tweetHalves[0])
		} else {
			thisTweet.DateTime, err = time.Parse(time.RFC3339, tweetHalves[0])
		}
		if err != nil {
			d.logger.Debugf("Error parsing time for tweet at %s from %s: %s", tweetHalves[0], twtxtURL, err)
			continue
		}

		tweets = append(tweets, thisTweet)
	}

	return tweets, nil
}
