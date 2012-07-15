package main

import (
	"code.google.com/p/gorilla/securecookie"
	"code.google.com/p/gorilla/sessions"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"musebot"
	"musebot/provider"
	"net/http"
	"os"
	"strconv"
)

var sessionStore sessions.Store
var jobIdGenerator = make(chan int)

func writeApiResponse(w http.ResponseWriter, ar musebot.ApiResponse) {
	b, err := json.Marshal(ar)
	if err != nil {
		return
	}
	w.Write(b)
}

func wrapApiError(err error) musebot.ErrorApiResponse {
	return musebot.ErrorApiResponse{err.Error()}
}

func getSession(r *http.Request) *sessions.Session {
	session, _ := sessionStore.Get(r, "musebot")
	return session
}

func isSessionBool(session *sessions.Session, whichBool string) bool {
	s, v := session.Values[whichBool]
	if !v {
		return false
	}
	p, n := s.(bool)
	if !n || !p {
		return false
	}
	return true
}

func isLoggedIn(session *sessions.Session) bool {
	return isSessionBool(session, "logged-in")
}

func isAdmin(session *sessions.Session) bool {
	return isSessionBool(session, "administrator")
}

func enforceLoggedIn(session *sessions.Session, w http.ResponseWriter) bool {
	if !isLoggedIn(session) {
		w.WriteHeader(http.StatusForbidden)
		writeApiResponse(w, wrapApiError(errors.New("You must be logged in to do that!")))
		return false
	}
	return true
}

func runHttpServer(cfg *musebot.JsonCfg) {
	if len(cfg.SessionStoreAuthKey) != 32 && len(cfg.SessionStoreAuthKey) != 64 {
		b64 := base64.StdEncoding
		log.Fatalln("SessionStoreAuthKey must be 32 or 64 bytes long, not", len(cfg.SessionStoreAuthKey), "bytes! Here's a suggested value:", b64.EncodeToString(securecookie.GenerateRandomKey(64)))
	}
	sessionStore = sessions.NewCookieStore(cfg.SessionStoreAuthKey)

	// let's try to get the default provider
	defaultProvider, wasFound := musebot.CurrentProviders[config.DefaultProvider]
	if !wasFound {
		for _, p := range musebot.CurrentProviders {
			defaultProvider = p
			break
		}
	}

	// job ID generator
	go func(generatorPipe chan int) {
		i := 0
		for {
			generatorPipe <- i
			i = i + 1
		}
	}(jobIdGenerator)

	var eProviderNotFound = errors.New("Provider not found")

	// GO GO WEB HANDLER
	http.HandleFunc("/api/current_song/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		currentSong, isPlaying, err := musebot.CurrentBackend.CurrentSong()
		if err != nil {
			writeApiResponse(w, wrapApiError(err))
		} else {
			if isPlaying {
				writeApiResponse(w, musebot.CurrentSongApiResponse{Playing: isPlaying, CurrentSong: &currentSong})
			} else {
				writeApiResponse(w, musebot.CurrentSongApiResponse{Playing: isPlaying, CurrentSong: nil})
			}
		}
	})

	http.HandleFunc("/api/playback_queue/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		playbackQueue, err := musebot.CurrentBackend.PlaybackQueue()
		if err != nil {
			writeApiResponse(w, wrapApiError(err))
		} else {
			writeApiResponse(w, musebot.PlaybackQueueApiResponse{playbackQueue})
		}
	})

	http.HandleFunc("/api/search_and_queue_first/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		// get the query!
		queryStrMap := r.URL.Query()
		queryArray, ok := queryStrMap["q"]
		if !ok || len(queryArray) < 1 || len(queryArray[0]) == 0 {
			writeApiResponse(w, wrapApiError(errors.New("You must pass a 'q' argument specifying the query!")))
			return
		}

		query := queryArray[0]
		// cool

		searchRes, err := musebot.CurrentProviders["provider.GroovesharkProvider"].Search(query)
		if err != nil {
			writeApiResponse(w, wrapApiError(err))
			return
		}

		if len(searchRes) == 0 {
			writeApiResponse(w, wrapApiError(errors.New("There were no results for that query.")))
			return
		}

		fetchChan := make(chan musebot.ProviderMessage, 1000)
		log.Println(searchRes[0].Title)

		go searchRes[0].Provider.FetchSong(&searchRes[0], fetchChan)

		waitToEnd := make(chan bool)

		go (func(fetchChan chan musebot.ProviderMessage, song *musebot.SongInfo, done chan bool) {
			for {
				msg := <-fetchChan
				log.Println(msg)
				if msg.Type == "done" {
					musebot.CurrentBackend.Add(*song)
					writeApiResponse(w, musebot.QueuedApiResponse{*song})
					done <- true
					return
				} else if msg.Type == "error" {
					writeApiResponse(w, wrapApiError(msg.Content.(error)))
					done <- true
					return
				}
			}
		})(fetchChan, &searchRes[0], waitToEnd)

		<-waitToEnd

	})

	http.HandleFunc("/api/available_providers/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		outputBlah := make(map[string]string)
		for k, v := range musebot.CurrentProviders {
			outputBlah[k] = v.Name()
		}
		writeApiResponse(w, musebot.AvailableProvidersApiResponse{outputBlah})
	})

	http.HandleFunc("/api/search/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		// get the query!
		queryStrMap := r.URL.Query()
		qArray, ok := queryStrMap["q"]
		if !ok || len(qArray) < 1 || len(qArray[0]) == 0 {
			writeApiResponse(w, wrapApiError(errors.New("You must pass a 'q' argument specifying the query!")))
			return
		}

		q := qArray[0]

		// now the provider
		providerArray, ok := queryStrMap["provider"]
		var provider provider.Provider
		if !ok || len(providerArray) < 1 || len(providerArray[0]) == 0 {
			provider = defaultProvider
		} else {
			providerName := providerArray[0]
			provider, ok = musebot.CurrentProviders[providerName]
			if !ok {
				writeApiResponse(w, wrapApiError(eProviderNotFound))
				return
			}
		}

		// now we have a provider, we can search!
		searchResults, err := provider.Search(q)
		if err != nil {
			writeApiResponse(w, wrapApiError(err))
			return
		}
		writeApiResponse(w, musebot.SearchResultsApiResponse{searchResults})
	})

	http.HandleFunc("/api/logout/", func(w http.ResponseWriter, r *http.Request) {
		sess := getSession(r)
		result := true
		if !isLoggedIn(sess) {
			result = false
		}

		// cool
		sess.Values = map[interface{}]interface{}{}
		sess.Save(r, w)

		writeApiResponse(w, musebot.LoggedOutApiResponse{result})
		return
	})

	http.HandleFunc("/api/quit/", func(w http.ResponseWriter, r *http.Request) {
		if !enforceLoggedIn(getSession(r), w) {
			return
		}

		log.Println()
		log.Println("Exiting on request!")
		os.Exit(0)
	})

	http.HandleFunc("/api/add_to_queue/", func(w http.ResponseWriter, r *http.Request) {
		sess := getSession(r)
		if !enforceLoggedIn(sess, w) {
			return
		}

		providerName := r.FormValue("provider")
		providerId := r.FormValue("provider_id")

		si := musebot.SongInfo{}
		si.ProviderName = providerName
		si.ProviderId = providerId
		provider, exists := musebot.CurrentProviders[providerName]

		if !exists {
			writeApiResponse(w, wrapApiError(errors.New("That provider doesn't exist.")))
		}

		si.Provider = provider

		log.Println("UPDATING SONG INFO")

		si.Provider.UpdateSongInfo(&si)

		log.Println(si)

		provMessage := make(chan musebot.ProviderMessage)
		quit := make(chan bool)
		go si.Provider.FetchSong(&si, provMessage)

		log.Println(si, provMessage)

		log.Println(sess.Values["username"])

		go func(quit chan bool, provMessage chan musebot.ProviderMessage, w http.ResponseWriter, s *musebot.SongInfo, user string) {
			hasQuit := false
			jobId := <-jobIdGenerator
			var m musebot.ProviderMessage
			for {
				m = <-provMessage
				if m.Type == "error" {
					if !hasQuit {
						writeApiResponse(w, wrapApiError(m.Content.(error)))
						quit <- true
						return // done
					}
				} else if m.Type == "stages" {
					if !hasQuit {
						// tell them that we're AWESOME
						if m.Content == 0 {
							log.Println("ORDERING BACKEND TO ADD", s)
							musebot.CurrentBackend.Add(*s)
							writeApiResponse(w, musebot.QueuedApiResponse{*s})
							quit <- true
							return // done
						} else {
							writeApiResponse(w, musebot.JobQueuedApiResponse{strconv.Itoa(jobId)})
							quit <- true
							hasQuit = true
						}
					}
				} else if m.Type == "done" && hasQuit {
					musebot.CurrentBackend.Add(*s)
				}
				if hasQuit {
					outputData := musebot.JobWebSocketApiResponse{JobId: strconv.Itoa(jobId), Data: m}
					jsonData, _ := json.Marshal(outputData)
					userOutputData := UserMessage{user: user, message: "JOB_DATA " + string(jsonData)}
					h.broadcastUser <- userOutputData
				}
			}
		}(quit, provMessage, w, &si, sess.Values["username"].(string))

		<-quit

	})

	http.HandleFunc("/api/login/", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			writeApiResponse(w, wrapApiError(errors.New("This method requires TLS! :<")))
			return
		}

		sess := getSession(r)

		username := r.FormValue("username")
		password := r.FormValue("password")

		a := musebot.CurrentAuthenticator
		result, user, err := a.CheckLogin(username, password)
		if err != nil {
			writeApiResponse(w, wrapApiError(err))
			return
		}
		if result {
			sess.Values["logged-in"] = true
			sess.Values["username"] = user.Username
			sess.Values["administrator"] = user.Administrator
			sess.Save(r, w)

			writeApiResponse(w, musebot.LoggedInApiResponse{username})
		} else {
			writeApiResponse(w, wrapApiError(errors.New("The username or password was incorrect.")))
		}

	})

	http.HandleFunc("/api/masquerade/", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			writeApiResponse(w, wrapApiError(errors.New("This method requires TLS! :<")))
			return
		}

		sess := getSession(r)

		if !enforceLoggedIn(sess, w) {
			return
		}

		if !isSessionBool(sess, "administrator") {
			writeApiResponse(w, wrapApiError(errors.New("You're not an administrator!")))
			return
		}

		queryStrMap := r.URL.Query()
		qArray, ok := queryStrMap["username"]
		if !ok || len(qArray) < 1 || len(qArray[0]) == 0 {
			writeApiResponse(w, wrapApiError(errors.New("You must pass a 'username' argument specifying the user to masquerade as!")))
			return
		}

		q := qArray[0]

		sess.Values["logged-in"] = true
		sess.Values["username"] = q
		sess.Values["administrator"] = true

		sess.Save(r, w)

		w.Write([]byte("YOU ARE NOW LOGGED IN AS "))
		w.Write([]byte(q))

	})

	registerWsHandler()

	if len(cfg.ListenAddr) != 0 {
		httpServer := &http.Server{Addr: cfg.ListenAddr}
		go func() {
			log.Fatalln(httpServer.ListenAndServe())
		}()
		log.Println(" - HTTP Server is listening on", cfg.ListenAddr)
	}

	if len(cfg.SslListenAddr) == 0 {
		log.Fatalln(" x The HTTPS server *must* run. Login will only take place over HTTPS.")
	}
	httpsServer := &http.Server{Addr: cfg.SslListenAddr}
	go func() {
		log.Fatalln(httpsServer.ListenAndServeTLS("ssl.pub.pem", "ssl.priv.pem"))
	}()
	log.Println(" - HTTPS Server is listening on", cfg.SslListenAddr)
}
