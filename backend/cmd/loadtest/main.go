package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type inbound struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

func main() {
	target := flag.String("url", "ws://localhost:8080/ws", "websocket URL")
	clients := flag.Int("clients", 20, "number of bot game clients")
	timeout := flag.Duration("timeout", 20*time.Second, "overall timeout")
	flag.Parse()

	if _, err := url.ParseRequestURI(*target); err != nil {
		log.Fatalf("invalid url: %v", err)
	}

	var ok atomic.Int64
	var failed atomic.Int64
	start := time.Now()
	deadline := time.Now().Add(*timeout)
	var wg sync.WaitGroup
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := runClient(*target, deadline); err != nil {
				failed.Add(1)
				log.Printf("client %d failed: %v", id, err)
				return
			}
			ok.Add(1)
		}(i + 1)
	}
	wg.Wait()

	fmt.Printf("clients=%d ok=%d failed=%d duration=%s\n", *clients, ok.Load(), failed.Load(), time.Since(start).Round(time.Millisecond))
	if failed.Load() > 0 {
		log.Fatal("load test failed")
	}
}

func runClient(target string, deadline time.Time) error {
	conn, _, err := websocket.DefaultDialer.Dial(target, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := readType(conn, "session:ready", deadline); err != nil {
		return err
	}
	if err := write(conn, message{Type: "bot:join", Payload: map[string]string{"level": "easy"}}); err != nil {
		return err
	}
	if err := readType(conn, "game:start", deadline); err != nil {
		return err
	}
	if err := write(conn, message{Type: "game:move", Payload: map[string]string{"from": "e2", "to": "e4", "promotion": "q"}}); err != nil {
		return err
	}
	if err := readType(conn, "game:update", deadline); err != nil {
		return err
	}
	if err := readType(conn, "game:update", deadline); err != nil {
		return err
	}
	return write(conn, message{Type: "game:resign"})
}

func write(conn *websocket.Conn, msg message) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, raw)
}

func readType(conn *websocket.Conn, expected string, deadline time.Time) error {
	_ = conn.SetReadDeadline(deadline)
	var msg inbound
	if err := conn.ReadJSON(&msg); err != nil {
		return err
	}
	if msg.Type != expected {
		return fmt.Errorf("expected %s, got %s", expected, msg.Type)
	}
	return nil
}
