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
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/xerrors"
)

type Config struct {
	mu             sync.RWMutex
	ServerConfig   ServerConfig   `toml:"server_config"`
	InstanceConfig InstanceConfig `toml:"instance_info"`
}

type ServerConfig struct {
	AdminPassword            string `toml:"admin_password"`
	AdminPasswordHash        string
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
}

// InstanceConfig holds the values that will be filled in on the landing page template.
type InstanceConfig struct {
	SiteName        string `toml:"site_name"`
	SiteURL         string `toml:"site_url"`
	SiteDescription string `toml:"site_description"`
	OwnerName       string `toml:"owner_name"`
	OwnerEmail      string `toml:"owner_email"`
}

func parseConfig(path string) (*Config, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, xerrors.Errorf("can't open config file: %w", err)
	}
	b, err := io.ReadAll(fd)
	if err != nil {
		return nil, xerrors.Errorf("can't read contents of config file: %w", err)
	}
	conf := Config{}
	_, err = toml.Decode(string(b), &conf)
	if err != nil {
		return nil, xerrors.Errorf("can't parse config file as toml: %w", err)
	}

	if conf.ServerConfig.AdminPassword == "please_change_me" || strings.TrimSpace(conf.ServerConfig.AdminPassword) == "" {
		return nil, xerrors.New("please set admin_password in the configuration file")
	}
	pHash, err := HashPass(conf.ServerConfig.AdminPassword)
	if err != nil {
		return nil, xerrors.Errorf("when hashing admin password: %w", err)
	}
	conf.ServerConfig.AdminPasswordHash = pHash
	conf.ServerConfig.AdminPassword = ""

	intervalParsed, err := time.ParseDuration(conf.ServerConfig.FetchIntervalStr)
	if err != nil {
		return nil, xerrors.Errorf("when parsing fetch interval: %w", err)
	}
	conf.ServerConfig.FetchInterval = intervalParsed

	msgLogFd, err := os.OpenFile(conf.ServerConfig.MessageLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, xerrors.Errorf("when opening message log file: %w", err)
	}
	conf.ServerConfig.MessageLogFd = msgLogFd
	log.SetOutput(conf.ServerConfig.MessageLogFd)

	reqLogFd, err := os.OpenFile(conf.ServerConfig.RequestLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, xerrors.Errorf("when opening request log file: %w", err)
	}
	conf.ServerConfig.RequestLogFd = reqLogFd

	return &conf, nil
}
