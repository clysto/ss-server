package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "0.0.0.0:8080", "http service address")

var upgrader = websocket.Upgrader{}

func init() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
}

type Sessions struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func (s *Sessions) run() {
	for {
		select {
		case conn := <-s.register:
			s.clients[conn] = true
		case conn := <-s.unregister:
			if _, ok := s.clients[conn]; ok {
				delete(s.clients, conn)
				conn.Close()
			}
		case message := <-s.broadcast:
			for conn := range s.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Println(err)
					conn.Close()
					delete(s.clients, conn)
				}
			}
		}
	}
}

func (s *Sessions) handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	s.register <- conn

	go func() {
		defer func() {
			s.unregister <- conn
		}()
		err := conn.WriteMessage(websocket.TextMessage, []byte{0})
		if err != nil {
			log.Println(err)
		}
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			s.broadcast <- message
		}
	}()
}

func main() {
	sessions := &Sessions{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go sessions.run()
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/", sessions.handle)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
