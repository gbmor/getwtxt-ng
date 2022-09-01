package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gbmor/getwtxt-ng/registry"
	log "github.com/sirupsen/logrus"
)

func InitTicker(t time.Duration, dbConn *registry.DB) chan<- struct{} {
	if err := pullAllTweets(dbConn); err != nil {
		log.Errorf("Error syncing: %s", err)
	}

	tick := time.NewTicker(t)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if err := pullAllTweets(dbConn); err != nil {
					log.Errorf("Error syncing: %s", err)
				}
			}
		}
	}()

	return done
}

func pullAllTweets(dbConn *registry.DB) error {
	begin := time.Now().UTC()
	log.Debugf("Initiating sync at %s", begin)
	defer log.Debugf("Sync finished after %s", time.Since(begin))

	ctx := context.Background()
	users, err := dbConn.GetAllUsers(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't get all users to sync tweets: %w", err)
	}

	usersSynced := make([]registry.User, 0, len(users))
	for i, e := range users {
		tweets, err := dbConn.FetchTwtxt(e.URL, e.ID, e.LastSync)
		if err != nil {
			log.Errorf("Couldn't get twtxt file for user %s: %s", e.URL, err)
			continue
		}
		if err := dbConn.InsertTweets(ctx, tweets); err != nil {
			return fmt.Errorf("couldn't insert tweets for user %s during sync: %w", e.URL, err)
		}
		users[i].LastSync = time.Now().UTC()
		usersSynced = append(usersSynced, users[i])
	}

	if err := dbConn.UpdateUsersSyncTime(ctx, usersSynced); err != nil {
		return fmt.Errorf("couldn't update users sync time: %w", err)
	}

	return nil
}
