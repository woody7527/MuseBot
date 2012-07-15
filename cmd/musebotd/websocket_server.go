package main

import (
	"code.google.com/p/go.net/websocket"
	"log"
	"net/http"
)

type UserMessage struct {
	user    string
	message string
}

type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan string

	broadcastUser chan UserMessage

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

var h = hub{
	broadcast:     make(chan string),
	broadcastUser: make(chan UserMessage),
	register:      make(chan *connection),
	unregister:    make(chan *connection),
	connections:   make(map[*connection]bool),
}

func safeClose(c chan string) {
	defer func() { recover() }()
	close(c)
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			delete(h.connections, c)
			//close(c.send)
			safeClose(c.send)
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					safeClose(c.send)
					go c.ws.Close()
				}
			}
		case m := <-h.broadcastUser:
			for c := range h.connections {
				if c.user != m.user {
					log.Println(c.user, m.user)
					continue
				}
				select {
				case c.send <- m.message:
				default:
					delete(h.connections, c)
					safeClose(c.send)
					go c.ws.Close()
				}
			}
		}
	}
}

type connection struct {
	ws   *websocket.Conn
	user string
	send chan string
}

func (c *connection) writer() {
	for message := range c.send {
		err := websocket.Message.Send(c.ws, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

func wsHandler(ws *websocket.Conn) {
	// check that they're logged in!
	httpRequest := ws.Request()
	session := getSession(httpRequest)
	if !isLoggedIn(session) {
		websocket.Message.Send(ws, "NOT_LOGGED_IN")
		ws.Close()
		return
	}

	c := &connection{send: make(chan string, 256), ws: ws, user: session.Values["username"].(string)}
	h.register <- c
	defer func() { h.unregister <- c }()
	c.writer()
}

func registerWsHandler() {
	http.Handle("/ws", websocket.Handler(wsHandler))

	go h.run()

	// also:
	go func(provider chan string, websocketbroadcast chan string) {
		for {
			websocketbroadcast <- <-provider
		}
	}(backendPipe, h.broadcast)
}
