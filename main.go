package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type ChatRoom struct {
	clients map[*Client]bool
	mutex   sync.Mutex
}

func (c *ChatRoom) broadcast(message []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for client := range c.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(c.clients, client)
		}
	}
}

func (c *ChatRoom) handleMessages() {
	for {
		for client := range c.clients {
			_, message, err := client.conn.ReadMessage()
			if err != nil {
				c.mutex.Lock()
				close(client.send)
				delete(c.clients, client)
				c.mutex.Unlock()
				continue
			}
			c.broadcast(message)
		}
	}
}

func (c *ChatRoom) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	c.clients[client] = true

	go func() {
		for {
			message, ok := <-client.send
			if !ok {
				return
			}
			err := client.conn.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				return
			}
		}
	}()
}

func main() {
	room := &ChatRoom{clients: make(map[*Client]bool)}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/ws", room.handleConnection)

	go room.handleMessages()

	fmt.Println("Server is running on :8080")
	http.ListenAndServe(":8080", nil)
}
