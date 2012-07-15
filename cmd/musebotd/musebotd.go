// Copyright 2012 Luke Granger-Brown. All rights reserved.

package main

import (
	"log"
	"math/rand"
	"musebot"
	"time"
)

var config *musebot.JsonCfg
var backendPipe chan string

func main() {
	log.Println("MuseBot is starting up!")
	log.Println("--- COPYRIGHT 2012 LUKE GRANGER-BROWN. ALL RIGHTS RESERVED. ---")
	log.Println()

	// Seed PRNG
	rand.Seed(time.Now().UnixNano())

	// Load configuration
	log.Println(" - Loading configuration...")
	config = &musebot.JsonCfg{}
	err := config.LoadConfiguration()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println()

	musebot.CurrentAuthenticator = setupAuthenticator(config)
	log.Println()

	musebot.CurrentBackend, backendPipe = setupPlaybackBackend(config)
	log.Println()

	musebot.CurrentProviders = setupSongProviders(config)
	log.Println()

	runHttpServer(config)

	for {
		time.Sleep(60 * time.Second)
	}
}
