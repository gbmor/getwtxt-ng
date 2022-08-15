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
	"net/http"
	"os"
	"time"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/ogier/pflag"
	log "github.com/sirupsen/logrus"
)

var flagConfig = pflag.StringP("config", "c", "getwtxt-ng.toml", "path to config file")

func main() {
	pflag.Parse()
	fmt.Printf("getwtxt-ng %s\n", common.Version)
	conf, err := readConfig(*flagConfig)
	if err != nil {
		fmt.Printf("Error loading configuration from %s: %s\n", *flagConfig, err)
		os.Exit(1)
	}
	if err := conf.parse(); err != nil {
		fmt.Printf("Config at %s has errors: %s\n", *flagConfig, err)
		os.Exit(1)
	}

	if conf.ServerConfig.DebugMode {
		log.SetLevel(log.DebugLevel)
	}
	log.SetOutput(conf.ServerConfig.MessageLogFd)

	signalWatcher(conf, log.StandardLogger())

	dbConn, err := registry.InitSQLite(conf.ServerConfig.DatabasePath,
		conf.ServerConfig.EntriesPerPageMax,
		conf.ServerConfig.EntriesPerPageMin,
		nil,
		log.StandardLogger())
	if err != nil {
		log.Errorf("Could not initialize database: %s", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	setUpRoutes(r, conf, dbConn)
	loggedHandler := handlers.CombinedLoggingHandler(conf.ServerConfig.RequestLogFd, r)

	var handler http.Handler
	if conf.ServerConfig.HTTPRequestsPerMinute > 0 {
		rl := getHTTPRateLimiter(conf)
		rateLimitedHandler := rl.RateLimit(loggedHandler)
		handler = rateLimitedHandler
	} else {
		handler = loggedHandler
	}

	s := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf("%s:%s", conf.ServerConfig.IP, conf.ServerConfig.Port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	err = s.ListenAndServe()
	log.Infof("%s", err)
}
