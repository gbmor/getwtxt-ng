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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	mu             sync.RWMutex
	ServerConfig   ServerConfig   `toml:"server_config"`
	InstanceConfig InstanceConfig `toml:"instance_info"`
}

type ServerConfig struct {
	AdminPassword            string `toml:"admin_password"`
	IP                       string `toml:"bind_ip"`
	Port                     string `toml:"port"`
	DatabasePath             string `toml:"database_path"`
	MessageLogPath           string `toml:"message_log"`
	MessageLogFd             *os.File
	RequestLogPath           string `toml:"request_log"`
	RequestLogFd             *os.File
	FetchIntervalStr         string `toml:"fetch_interval"`
	FetchInterval            time.Duration
	AssetsDirectoryPath      string `toml:"assets_directory"`
	StaticFilesDirectoryPath string `toml:"static_files_directory"`
	EntriesPerPageMax        int    `toml:"entries_per_page_max"`
	EntriesPerPageMin        int    `toml:"entries_per_page_min"`
	HTTPRequestsPerMinute    int    `toml:"http_requests_per_minute"`
	HTTPRequestsBurstMax     int    `toml:"http_requests_max_burst"`
	DebugMode                bool   `toml:"debug_mode"`
}

// InstanceConfig holds the values that will be filled in on the landing page template.
type InstanceConfig struct {
	SiteName        string `toml:"site_name"`
	SiteURL         string `toml:"site_url"`
	SiteDescription string `toml:"site_description"`
	OwnerName       string `toml:"owner_name"`
	OwnerEmail      string `toml:"owner_email"`
}

// Reads the config file directly into a *Config without doing any additional parsing.
func readConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("can't read contents of config file: %w", err)
	}
	conf := Config{}
	_, err = toml.Decode(string(b), &conf)
	if err != nil {
		return nil, fmt.Errorf("can't parse config file as toml: %w", err)
	}
	return &conf, nil
}

// Open files, parse fetch interval, hash admin pass
func (c *Config) parse() error {
	if strings.TrimSpace(c.ServerConfig.AdminPassword) == "" {
		return errors.New("please set admin_password in the configuration file")
	}

	if c.ServerConfig.EntriesPerPageMax < 20 {
		c.ServerConfig.EntriesPerPageMax = 20
	}
	if c.ServerConfig.EntriesPerPageMin < 10 {
		c.ServerConfig.EntriesPerPageMin = 10
	}

	intervalParsed, err := time.ParseDuration(c.ServerConfig.FetchIntervalStr)
	if err != nil {
		return fmt.Errorf("when parsing fetch interval: %w", err)
	}
	c.ServerConfig.FetchInterval = intervalParsed

	msgLogFd, err := os.OpenFile(c.ServerConfig.MessageLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("when opening message log file: %w", err)
	}
	c.ServerConfig.MessageLogFd = msgLogFd

	reqLogFd, err := os.OpenFile(c.ServerConfig.RequestLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("when opening request log file: %w", err)
	}
	c.ServerConfig.RequestLogFd = reqLogFd

	return nil
}

// Reloads "safe" configuration options.
// To be called on SIGHUP.
func (c *Config) reload(path string, logger *log.Logger) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	newConf, err := readConfig(path)
	if err != nil {
		return fmt.Errorf("while reloading config: %w", err)
	}

	if strings.TrimSpace(newConf.ServerConfig.AdminPassword) == "" {
		return errors.New("please set admin_password in the configuration file")
	}

	if newConf.ServerConfig.MessageLogPath != c.ServerConfig.MessageLogPath {
		msgLogFd, err := os.OpenFile(newConf.ServerConfig.MessageLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			logger.Infof("When opening new message log file on config reload: %s", err)
		} else {
			oldMsgLogFd := c.ServerConfig.MessageLogFd
			c.ServerConfig.MessageLogFd = msgLogFd
			c.ServerConfig.MessageLogPath = newConf.ServerConfig.MessageLogPath
			log.SetOutput(c.ServerConfig.MessageLogFd)
			if err := oldMsgLogFd.Close(); err != nil {
				logger.Infof("When closing old message log fd on config reload: %s", err)
			}
		}
	}

	fetchInterval, err := time.ParseDuration(newConf.ServerConfig.FetchIntervalStr)
	if err != nil {
		logger.Infof("Couldn't parse new fetch interval when reloading config: %s", err)
	} else {
		c.ServerConfig.FetchInterval = fetchInterval
	}

	c.ServerConfig.AssetsDirectoryPath = newConf.ServerConfig.AssetsDirectoryPath
	c.ServerConfig.StaticFilesDirectoryPath = newConf.ServerConfig.StaticFilesDirectoryPath
	c.ServerConfig.EntriesPerPageMax = newConf.ServerConfig.EntriesPerPageMax
	c.ServerConfig.EntriesPerPageMin = newConf.ServerConfig.EntriesPerPageMin
	c.InstanceConfig = newConf.InstanceConfig

	if c.ServerConfig.EntriesPerPageMax < 20 {
		c.ServerConfig.EntriesPerPageMax = 20
	}
	if c.ServerConfig.EntriesPerPageMin < 10 {
		c.ServerConfig.EntriesPerPageMin = 10
	}

	if c.ServerConfig.DebugMode {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	return nil
}
