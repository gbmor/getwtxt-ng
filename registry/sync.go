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
	"net/http"
	"strings"

	"github.com/gbmor/getwtxt-ng/common"
	"golang.org/x/xerrors"
)

func (d *DB) FetchTwtxt(twtxtURL string) (*http.Response, error) {
	if !common.IsValidURL(twtxtURL) {
		return nil, xerrors.Errorf("invalid URL provided: %s", twtxtURL)
	}

	req, err := http.NewRequest("GET", twtxtURL, nil)
	if err != nil {
		return nil, xerrors.Errorf("couldn't create http request to fetch %s: %w", twtxtURL, err)
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, xerrors.Errorf("error making http request to %s: %w", twtxtURL, err)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		return nil, xerrors.Errorf("received non-text/plain content type from %s: %s", twtxtURL, contentType)
	}

	return resp, nil
}
