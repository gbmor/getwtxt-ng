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
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestInitDB(t *testing.T) {
	db, err := InitSQLite(":memory:", 20, 1000, nil, log.StandardLogger())
	if err != nil {
		t.Error(err.Error())
	}
	defer func() {
		if err := db.conn.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	t.Run("in-memory, check for tables", func(t *testing.T) {
		rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
		if err != nil {
			t.Error(err.Error())
		}
		defer func() {
			_ = rows.Close()
		}()
		tables := make([]string, 0)
		for rows.Next() {
			tbl := ""
			if err := rows.Scan(&tbl); err != nil {
				t.Error(err.Error())
			}
			tables = append(tables, tbl)
		}
		if tables[1] != "tweets" && tables[2] != "users" {
			t.Errorf("Got unexpected table names: %v", tables)
		}
	})
}
