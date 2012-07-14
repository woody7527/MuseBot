// Copyright 2012 Luke Granger-Brown. All rights reserved.

package main

import (
	"log"
	"musebot"
	"musebot/auth"
	"musebot/backend"
	"reflect"
	"time"
)

func setupAuthenticator(config *musebot.JsonCfg) auth.Authenticator {
	// Enumerate authenticators
	log.Println("Available auth backends:")
	authenticators := auth.Authenticators()
	authenticatorsMap := make(map[string]auth.Authenticator)
	for i := 0; i < len(authenticators); i++ {
		log.Println("*", authenticators[i], "("+reflect.TypeOf(authenticators[i]).String()[1:]+")")
		authenticatorsMap[reflect.TypeOf(authenticators[i]).String()[1:]] = authenticators[i]
	}

	// Select backend
	log.Println("Selected auth backend is", config.AuthBackend)
	authBackend, ok := authenticatorsMap[config.AuthBackend]
	if !ok {
		log.Fatalln("Backend not found! Double-check the config file against the list above!")
	}

	log.Println()
	log.Println("Using authenticator", authBackend)
	authBackend.Setup(config.AuthBackendConfig[config.AuthBackend])

	return authBackend
}

func setupPlaybackBackend(config *musebot.JsonCfg) (backend.Backend, chan string) {

	// Enumerate backends
	log.Println("Available backends:")
	backends := backend.Backends()
	backendsMap := make(map[string]backend.Backend)
	for i := 0; i < len(backends); i++ {
		log.Println("*", backends[i], "("+reflect.TypeOf(backends[i]).String()[1:]+")")
		backendsMap[reflect.TypeOf(backends[i]).String()[1:]] = backends[i]
	}

	// Select backend
	log.Println("Selected backend is", config.Backend)
	backend, ok := backendsMap[config.Backend]
	if !ok {
		log.Fatalln("Backend not found! Double-check the config file against the list above!")
	}

	log.Println()
	log.Println("Using backend", backend)

	backendPipe := make(chan string)
	backend.Setup(config.BackendConfig[config.Backend], backendPipe)

	return backend, backendPipe
}

func main() {
	log.Println("MuseBot is starting up!")
	log.Println("--- COPYRIGHT 2012 LUKE GRANGER-BROWN. ALL RIGHTS RESERVED. ---")
	log.Println()

	// Load configuration
	log.Println("Loading configuration...")
	config := &musebot.JsonCfg{}
	err := config.LoadConfiguration()
	if err != nil {
		log.Fatalln(err)
	}

	authenticator := setupAuthenticator(config)

	backend, backendChan := setupPlaybackBackend(config)

	log.Println(authenticator, backend, backendChan)

	backend.Add(musebot.SongInfo{
		MusicUrl: "/home/lukegb/music/Various - 2008 - Dr. Horrible's Sing-Along Blog Soundtrack/09 - Brand New Day.flac",
	})

	for {
		log.Println(backend.PlaybackQueue())
		time.Sleep(5 * time.Second)
	}
}
