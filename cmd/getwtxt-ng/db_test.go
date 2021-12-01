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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// getDBMocker gives us the SQL DB mocker.
// We can then induce error conditions easily in tests.
// Use an in-memory SQLite DB when the data is important.
func getDBMocker(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	dbConn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err.Error())
	}
	return dbConn, mock
}

func Test_initDB(t *testing.T) {
	t.Run("in-memory, check for tables", func(t *testing.T) {
		db, err := initDB(":memory:")
		if err != nil {
			t.Error(err.Error())
		}
		rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
		if err != nil {
			t.Error(err.Error())
		}
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
