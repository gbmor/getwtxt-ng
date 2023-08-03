package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"

	"github.com/gbmor/getwtxt-ng/common"
	"github.com/gbmor/getwtxt-ng/registry"
)

var flagConfig = flag.String("config", "getwtxt-ng.toml", "Path to getwtxt-ng's config file")

func main() {
	flag.Parse()
	binaryName := os.Args[0]
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Please specify the path to the user list as an argument:")
		fmt.Printf("\t%s /path/to/user_list.txt\n", binaryName)
		os.Exit(1)
	}

	confFile, err := os.ReadFile(*flagConfig)
	if err != nil {
		fmt.Printf("Could not read getwtxt-ng config file at %s: %s", *flagConfig, err)
		fmt.Printf("You may need to specify an alternate config path with:\n")
		fmt.Printf("\t%s -config /path/to/getwtxt-ng.toml /path/to/user_list.txt\n", binaryName)
		os.Exit(1)
	}

	conf := struct {
		ServerConfig struct {
			DatabasePath string `toml:"database_path"`
		} `toml:"server_config"`
		InstanceInfo struct {
			SiteURL  string `toml:"site_url"`
			SiteName string `toml:"site_name"`
		} `toml:"instance_info"`
	}{}
	if _, err := toml.Decode(string(confFile), &conf); err != nil {
		fmt.Printf("Could not grab database path from getwtxt-ng config file at %s: %s\n", *flagConfig, err)
		os.Exit(1)
	}
	if conf.ServerConfig.DatabasePath == "" {
		fmt.Printf("Could not grab database path from getwtxt-ng config file at %s: %s\n", *flagConfig, err)
		os.Exit(1)
	}

	userAgent := fmt.Sprintf("getwtxt-ng/%s (+%s; @getwtxt-ng/init-bulk-follow )", common.Version, conf.InstanceInfo.SiteURL)
	dbConn, err := registry.InitSQLite(conf.ServerConfig.DatabasePath, 10, 10, nil, userAgent, log.StandardLogger())
	if err != nil {
		fmt.Printf("Could not connect to database at %s: %s\n", conf.ServerConfig.DatabasePath, err)
		os.Exit(1)
	}

	filePath := args[0]
	userFile, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Couldn't open user list: %s\n", err)
		os.Exit(1)
	}

	usersToAdd := make([]registry.User, 0, 5)
	ctx := context.Background()

	bodyScanner := bufio.NewScanner(userFile)
	for bodyScanner.Scan() {
		line := bodyScanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// This is to prevent variations of the same URL showing up multiple times.
		// Eg: http://example.com/twtxt.txt vs https://example.com/twtxt.txt
		// We're also chomping www. off.
		parsedURL, err := url.Parse(fields[1])
		if err != nil {
			log.Errorf("couldn't parse %s as URL: %s", fields[1], err)
			continue
		}
		host := strings.TrimPrefix(parsedURL.Host, "www.")
		constructedURL := fmt.Sprintf("%s%s", host, parsedURL.Path)

		userSearchOut, err := dbConn.SearchUsers(ctx, 1, 10, constructedURL)
		if err != nil {
			log.Errorf("While searching for user %s: %s", fields[1], err)
			continue
		}
		if len(userSearchOut) > 0 {
			continue
		}
		var dt time.Time
		if len(fields) < 3 {
			dt = time.Now().UTC()
		} else {
			dt, err = time.Parse(time.RFC3339, fields[2])
			if err != nil {
				dt, err = time.Parse(time.RFC3339Nano, fields[2])
				if err != nil {
					continue
				}
			}
		}

		thisUser := registry.User{
			Nick:          fields[0],
			URL:           fields[1],
			DateTimeAdded: dt,
		}
		usersToAdd = append(usersToAdd, thisUser)
	}

	users, err := dbConn.InsertUsers(ctx, usersToAdd)
	if err != nil {
		fmt.Printf("When bulk inserting users: %s", err)
		os.Exit(1)
	}

	for i, user := range users {
		tweets, err := dbConn.FetchTwtxt(user.URL, user.ID, time.Time{})
		if err != nil {
			log.Errorf("Couldn't fetch tweets for %s: %s", user.URL, err)
			continue
		}
		err = dbConn.InsertTweets(ctx, tweets)
		if err != nil {
			log.Errorf("Couldn't fetch tweets for %s: %s", user.URL, err)
			continue
		}
		users[i].LastSync = time.Now().UTC()
	}

	plainUsersResp := registry.FormatUsersPlain(users)
	fmt.Printf("Successfully added the following users:\n\n")
	fmt.Printf("%s\n", plainUsersResp)
}
