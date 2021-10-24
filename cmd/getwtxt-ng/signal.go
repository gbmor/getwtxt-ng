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
	"log"
	"os"
	"os/signal"
)

func watchForInterrupt(conf *Config) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for sigint := range c {
			conf.mu.Lock()
			log.Printf("Caught %v\n", sigint)
			log.Println("Closing log files and switching to stderr")
			if err := conf.ServerConfig.MessageLogFd.Close(); err != nil {
				log.Printf("When closing message log: %s\n", err)
			}
			if err := conf.ServerConfig.RequestLogFd.Close(); err != nil {
				log.Printf("When closing request log: %s\n", err)
			}
			os.Exit(130)
		}
	}()
}
