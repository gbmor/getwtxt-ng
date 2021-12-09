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
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestDB_FetchTwtxt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(twtxtTestingHandler))
	client := srv.Client()
	client.Timeout = 1 * time.Second

	type args struct {
		twtxtURL     string
		userID       string
		lastModified time.Time
	}
	tests := []struct {
		name          string
		db            *DB
		args          args
		want          []Tweet
		wantErr       bool
		skipDeepEqual bool
	}{
		{
			name:    "empty url",
			db:      &DB{},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil http client",
			db:   &DB{},
			args: args{
				twtxtURL: "https://example.org/twtxt.txt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "304 not modified",
			db: &DB{
				Client: client,
			},
			args: args{
				twtxtURL: fmt.Sprintf("%s/twtxt/304", srv.URL),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "404 not found",
			db: &DB{
				Client: client,
			},
			args: args{
				twtxtURL: fmt.Sprintf("%s/twtxt/404", srv.URL),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "content-type other than text/plain",
			db: &DB{
				Client: client,
			},
			args: args{
				twtxtURL: fmt.Sprintf("%s/twtxt/json", srv.URL),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "success with soft fail",
			db: &DB{
				Client: client,
			},
			args: args{
				twtxtURL: fmt.Sprintf("%s/twtxt.txt", srv.URL),
			},
			want:          nil,
			wantErr:       false,
			skipDeepEqual: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.db.FetchTwtxt(tt.args.twtxtURL, tt.args.userID, tt.args.lastModified)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchTwtxt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.skipDeepEqual && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FetchTwtxt() got = %v, want %v", got, tt.want)
			}
		})
	}
}
