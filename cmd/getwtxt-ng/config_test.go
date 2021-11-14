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
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"
)

func Test_readConfig(t *testing.T) {
	t.Run("fail to open config file", func(t *testing.T) {
		b := make([]byte, 10)
		_, err := rand.Read(b)
		if err != nil {
			t.Error(err)
		}
		badPath := fmt.Sprintf("/tmp/%x", b)
		_, err = readConfig(badPath)
		if err == nil {
			t.Errorf("Expected error when opening %s", badPath)
		}
		if !strings.Contains(err.Error(), "no such file or directory") {
			t.Errorf("Got other error than no such file or directory for %s :: %s", badPath, err)
		}
	})
	t.Run("fail to parse config file (not toml)", func(t *testing.T) {
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		fd.Write([]byte("invalid config file here"))
		fd.Close()
		_, err = readConfig(tmpFilePath)
		if err == nil {
			t.Errorf("Expected error when opening %s", tmpFilePath)
		}
		if !strings.Contains(err.Error(), "can't parse config file as toml") {
			t.Errorf("Got other error than can't parse config file as toml for %s :: %s", tmpFilePath, err)
		}
	})
	t.Run("successfully read a config file as toml", func(t *testing.T) {
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := "[server_config]\nbind_ip = \"127.0.0.1\""
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		if conf.ServerConfig.IP != "127.0.0.1" {
			t.Errorf("Got %s, expected 127.0.0.1", conf.ServerConfig.IP)
		}
	})
}

func TestConfig_parse(t *testing.T) {
	t.Run("unset admin_password", func(t *testing.T) {
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := "[server_config]\nbind_ip = \"127.0.0.1\""
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		err = conf.parse()
		if !strings.Contains(err.Error(), "please set admin_password") {
			t.Errorf("Expected error regarding unset admin_password, got: %s", err)
		}
	})
	t.Run("invalid fetch interval", func(t *testing.T) {
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := "[server_config]\nadmin_password = \"hunter2\"\nfetch_interval = \"3kg\""
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		err = conf.parse()
		if !strings.Contains(err.Error(), "fetch interval") {
			t.Errorf("Expected error parsing fetch interval, got: %s", err)
		}
	})
	t.Run("bad message log path", func(t *testing.T) {
		b := make([]byte, 10)
		_, err := rand.Read(b)
		if err != nil {
			t.Error(err)
		}
		reqLogPath := fmt.Sprintf("/tmp/%x", b)
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := fmt.Sprintf("[server_config]\nadmin_password = \"hunter2\"\nfetch_interval = \"1h\"\nrequest_log = \"%s\"", reqLogPath)
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		err = conf.parse()
		if !strings.Contains(err.Error(), "when opening message log") {
			t.Errorf("Expected error opening message log, got: %s", err)
		}
	})
	t.Run("bad request log path", func(t *testing.T) {
		b := make([]byte, 10)
		_, err := rand.Read(b)
		if err != nil {
			t.Error(err)
		}
		msgLogPath := fmt.Sprintf("/tmp/%x", b)
		defer os.Remove(msgLogPath)
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := fmt.Sprintf("[server_config]\nadmin_password = \"hunter2\"\nfetch_interval = \"1h\"\nmessage_log = \"%s\"", msgLogPath)
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		err = conf.parse()
		if !strings.Contains(err.Error(), "when opening request log") {
			t.Errorf("Expected error opening request log, got: %s", err)
		}
	})
	t.Run("successfully parse config values", func(t *testing.T) {
		b := make([]byte, 10)
		_, err := rand.Read(b)
		if err != nil {
			t.Error(err)
		}
		msgLogPath := fmt.Sprintf("/tmp/%x", b)
		defer os.Remove(msgLogPath)
		b = make([]byte, 10)
		_, err = rand.Read(b)
		if err != nil {
			t.Error(err)
		}
		reqLogPath := fmt.Sprintf("/tmp/%x", b)
		defer os.Remove(reqLogPath)
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := fmt.Sprintf("[server_config]\nadmin_password = \"hunter2\"\nfetch_interval = \"1h\"\nmessage_log = \"%s\"\nrequest_log = \"%s\"", msgLogPath, reqLogPath)
		fd.Write([]byte(contents))
		fd.Close()
		conf, err := readConfig(tmpFilePath)
		if err != nil {
			t.Error(err.Error())
		}
		if err := conf.parse(); err != nil {
			t.Error(err.Error())
		}
	})
}

func TestConfig_reload(t *testing.T) {
	t.Run("fail to open config file", func(t *testing.T) {
		b := make([]byte, 8)
		rand.Read(b)
		fnPath := fmt.Sprintf("%s/%x", os.TempDir(), b)
		oldConf := &Config{}
		if err := oldConf.reload(fnPath); !strings.Contains(err.Error(), "while reloading config") {
			t.Errorf("Expected error opening file, got %s", err)
		}
	})
	t.Run("complete with soft fails", func(t *testing.T) {
		oldConf := &Config{
			ServerConfig: ServerConfig{
				MessageLogPath: "/tmp/foo",
			},
		}
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := "[server_config]\nbind_ip = \"127.0.0.1\""
		fd.Write([]byte(contents))
		fd.Close()
		if err := oldConf.reload(tmpFilePath); err != nil {
			t.Error(err.Error())
		}
	})
	t.Run("complete", func(t *testing.T) {
		oldConf := &Config{
			ServerConfig: ServerConfig{
				MessageLogPath: "/tmp/foo",
			},
		}
		fd, err := os.CreateTemp(os.TempDir(), "getwtxt-ng-test-config")
		if err != nil {
			t.Errorf("When creating temp file: %s", err)
		}
		tmpFilePath := fd.Name()
		defer os.Remove(tmpFilePath)
		contents := `[server_config]
						bind_ip = "127.0.0.1"
						message_log = "message.log"
						fetch_interval = "1h"`
		fd.Write([]byte(contents))
		fd.Close()
		if err := oldConf.reload(tmpFilePath); err != nil {
			t.Error(err.Error())
		}
	})
}
