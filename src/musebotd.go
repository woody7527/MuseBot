// Copyright 2012 Luke Granger-Brown. All rights reserved.

package main

import (
	"log"
	"musebot"
	"musebot/auth"
	"musebot/backend"
	"musebot/provider"
	"reflect"
	"time"
)

func setupAuthenticator(config *musebot.JsonCfg) auth.Authenticator {
	// Enumerate authenticators
	log.Println(" - Available auth backends:")
	authenticators := auth.Authenticators()
	authenticatorsMap := make(map[string]auth.Authenticator)
	for i := 0; i < len(authenticators); i++ {
		authenticatorName := reflect.TypeOf(authenticators[i]).String()[1:]
		log.Println("   *", authenticators[i], "("+authenticatorName+")")
		authenticatorsMap[authenticatorName] = authenticators[i]
	}

	// Select backend
	log.Println(" - Selected auth backend is", config.AuthBackend)
	authBackend, ok := authenticatorsMap[config.AuthBackend]
	if !ok {
		log.Fatalln("Backend not found! Double-check the config file against the list above!")
	}

	log.Println(" - Using authenticator", authBackend)
	authBackend.Setup(config.AuthBackendConfig[config.AuthBackend])

	return authBackend
}

func setupPlaybackBackend(config *musebot.JsonCfg) (backend.Backend, chan string) {

	// Enumerate backends
	log.Println(" - Available backends:")
	backends := backend.Backends()
	backendsMap := make(map[string]backend.Backend)
	for i := 0; i < len(backends); i++ {
		backendName := reflect.TypeOf(backends[i]).String()[1:]
		log.Println("   *", backends[i], "("+backendName+")")
		backendsMap[backendName] = backends[i]
	}

	// Select backend
	log.Println(" - Selected backend is", config.Backend)
	backend, ok := backendsMap[config.Backend]
	if !ok {
		log.Fatalln(" x Backend not found! Double-check the config file against the list above!")
	}
	log.Println(" - Using backend", backend)

	backendPipe := make(chan string)
	backend.Setup(config.BackendConfig[config.Backend], backendPipe)

	return backend, backendPipe
}

func setupSongProviders(config *musebot.JsonCfg) map[string]provider.Provider {
	// Enumerate providers...
	log.Println(" - Available providers:")
	providers := provider.Providers()
	providersMap := make(map[string]provider.Provider)
	for i := 0; i < len(providers); i++ {
		prv := providers[i]
		providerName := reflect.TypeOf(prv).String()[1:]
		log.Println("   *", prv, "("+providerName+")")
		providersMap[providerName] = prv

		err := prv.Setup(config.ProviderBackendConfig[providerName])

		if err != nil {
			log.Println("     ! There was an error enabling the provider:", err)
			log.Println("       The provider has been disabled.")
			delete(providersMap, providerName)
		} else {
			log.Println("     o OK!")
		}
	}

	if len(providersMap) == 0 {
		log.Fatalln("   x No providers were available!")
	}

	return providersMap
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
	log.Println()

	backend, backendChan := setupPlaybackBackend(config)
	log.Println()

	providers := setupSongProviders(config)
	log.Println()

	log.Println(authenticator, backend, backendChan, providers)

	for {
		//log.Println(backend.PlaybackQueue())
		time.Sleep(5 * time.Second)
	}
}
