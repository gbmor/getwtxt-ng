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
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/ogier/pflag"
)

var flagConfig = pflag.StringP("config", "c", "getwtxt-ng.toml", "path to config file")

func main() {
	pflag.Parse()
	conf, err := readConfig(*flagConfig)
	if err != nil {
		fmt.Printf("Error loading configuration from %s: %s\n", *flagConfig, err)
		os.Exit(1)
	}
	if err := conf.parse(); err != nil {
		fmt.Printf("Config at %s has errors: %s\n", *flagConfig, err)
		os.Exit(1)
	}
	log.SetOutput(conf.ServerConfig.MessageLogFd)
	signalWatcher(conf)

	r := mux.NewRouter()
	setUpRoutes(r, conf)
	s := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("%s:%s", conf.ServerConfig.IP, conf.ServerConfig.Port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	log.Println(s.ListenAndServe())
}
