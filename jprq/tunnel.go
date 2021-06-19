package jprq

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math/rand"
	"time"
)

type Tunnel struct {
	host           string
	conn           *websocket.Conn
	token          string
	requests       map[uuid.UUID]RequestMessage
	requestChan    chan RequestMessage
	responseChan   chan ResponseMessage
	numOfReqServed int
}

func (j Jprq) GetTunnelByHost(host string) (*Tunnel, error) {
	t, ok := j.tunnels[host]

	if !ok {
		return t, errors.New("Tunnel doesn't exist")
	}

	return t, nil
}
func (j Jprq) GetUnusedHost(host, subdomain string) string {
	if _, err := j.GetTunnelByHost(host); err == nil {
		rand.Seed(time.Now().UnixNano())
		min := 0
		max := len(Adjectives)
		hostPrefix := fmt.Sprintf("%s-%s", Adjectives[rand.Intn(max-min)+min], subdomain)
		host = fmt.Sprintf("%s.%s", hostPrefix, j.baseHost)
		host = j.GetUnusedHost(host, subdomain)
	}
	return host
}
func (j *Jprq) AddTunnel(host string, conn *websocket.Conn) *Tunnel {
	token, _ := uuid.NewV4()
	requests := make(map[uuid.UUID]RequestMessage)
	requestChan := make(chan RequestMessage)
	tunnel := Tunnel{
		host:        host,
		conn:        conn,
		token:       token.String(),
		requests:    requests,
		requestChan: requestChan,
	}

	log.Println("New Tunnel: ", host)
	j.tunnels[host] = &tunnel
	return &tunnel
}

func (j *Jprq) DeleteTunnel(host string) {
	tunnel, ok := j.tunnels[host]
	if !ok {
		return
	}
	log.Printf("Deleted Tunnel: %s, Number Of Requests Served: %d", host, tunnel.numOfReqServed)
	close(tunnel.requestChan)
	delete(j.tunnels, host)
}

func (tunnel *Tunnel) DispatchRequests() {

	for {
		select {
		case requestMessage, more := <-tunnel.requestChan:
			if !more {
				return
			}
			messageContent, _ := bson.Marshal(requestMessage)
			tunnel.requests[requestMessage.ID] = requestMessage
			tunnel.conn.WriteMessage(websocket.BinaryMessage, messageContent)
		}
	}
}
