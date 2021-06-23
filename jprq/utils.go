package jprq

import (
	"github.com/gorilla/websocket"
	"time"
)

func keepAlive(ws *websocket.Conn, timeout time.Duration) {
	lastResponse := time.Now()
	ws.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})
	go func() {
		for {
			err := ws.WriteMessage(websocket.PingMessage, []byte("ping"))
			if err != nil {
				return
			}
			time.Sleep(timeout / 2)
			if time.Since(lastResponse) > timeout {
				ws.Close()
				return
			}
		}
	}()
}

var Adjectives = []string{
	"amazing", "ambitious", "amusing", "awesome",
	"brave", "bright", "broad-minded",
	"calm", "clever", "charming", "considerate", "confident", "courageous", "creative",
	"dazzling", "decisive", "determined", "diligent", "disciplined",
	"eager", "easygoing", "emotional", "energetic", "enthusiastic", "enchanting",
	"fabulous", "faithful", "fantastic", "fearless", "forceful", "frank", "friendly", "funny",
	"generous", "glorious", "gentle", "good",
	"hard-working", "helpful", "honest", "humorous",
	"imaginative", "independent", "ingenious", "intellectual", "intelligent", "intuitive", "inventive",
	"kind", "loving", "loyal", "modest", "nice", "optimistic",
	"passionate", "patient", "perfect", "persistent", "pioneering", "polite", "powerful", "practical",
	"quick-witted", "quiet",
	"rational", "reliable", "reserved", "resourceful", "romantic",
	"smart", "shy", "sincere", "sociable", "sympathetic",
	"talented", "thoughtful", "understanding", "versatile",
	"warmhearted", "wise", "willing", "witty", "wonderful",
}
