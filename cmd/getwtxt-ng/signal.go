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
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func signalWatcher(conf *Config, logger *log.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP)

	go func() {
		for sig := range c {
			switch sig {
			case syscall.SIGINT:
				conf.mu.Lock()
				logger.Infof("Caught %s\n", sig)
				logger.Info("Closing log files and switching to stderr")
				logger.SetOutput(os.Stderr)

				if err := conf.ServerConfig.MessageLogFd.Close(); err != nil {
					logger.Infof("When closing message log: %s\n", err)
				}
				if err := conf.ServerConfig.RequestLogFd.Close(); err != nil {
					logger.Infof("When closing request log: %s\n", err)
				}

				os.Exit(130)

			case syscall.SIGHUP:
				logger.Infof("Caught %s: reloading configuration...\n", sig)
				if err := conf.reload(*flagConfig, logger); err != nil {
					logger.Infof(err.Error())
				}
			}
		}
	}()
}
