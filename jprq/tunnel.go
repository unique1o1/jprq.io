package jprq

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Socket struct {
	sync.Mutex
	*websocket.Conn
}

func (c *Socket) WriteMessage(messageType int, data []byte) error {
	c.Lock()
	defer c.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

type Tunnel struct {
	host                     string
	conn                     *Socket
	token                    string
	requestsTracker          sync.Map
	requestChan              chan RequestMessage
	requestChanCloseNotifier chan struct{}
	numOfReqServed           int
}

func (j *Jprq) GetTunnelByHost(host string) (*Tunnel, error) {
	t, ok := j.tunnels.Load(host)
	if !ok {
		return nil, errors.New("Tunnel doesn't exist")
	}

	return t.(*Tunnel), nil
}
func (j *Jprq) GetUnusedHost(host, subdomain string) string {
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
func (j *Jprq) AddTunnel(host string, conn *Socket) *Tunnel {
	token, _ := uuid.NewV4()
	requestChan := make(chan RequestMessage)
	tunnel := Tunnel{
		host:                     host,
		conn:                     conn,
		token:                    token.String(),
		requestsTracker:          sync.Map{},
		requestChan:              requestChan,
		requestChanCloseNotifier: make(chan struct{}),
	}

	log.Println("New Tunnel: ", host)
	j.tunnels.Store(host, &tunnel)
	return &tunnel
}

func (j *Jprq) DeleteTunnel(host string) {
	t, ok := j.tunnels.Load(host)
	if !ok {
		return
	}
	tunnel := t.(*Tunnel)
	log.Printf("Deleted Tunnel: %s, Number Of Requests Served: %d", host, tunnel.numOfReqServed)
	close(tunnel.requestChanCloseNotifier) //close requestChanCloseNotifier to notify all requestChan sender to stop
	close(tunnel.requestChan)              // close request chan

	tunnel.requestsTracker.Range(func(key, value interface{}) bool {
		ch := value.(chan ResponseMessage)
		close(ch)
		tunnel.requestsTracker.Delete(key)
		return true
	})
	j.tunnels.Delete(host)
}

func (tunnel *Tunnel) DispatchRequests() {
	// Sent message to client
	for {
		select {
		case requestMessage, more := <-tunnel.requestChan:

			if !more {
				return
			}
			messageContent, _ := bson.Marshal(requestMessage)
			tunnel.conn.WriteMessage(websocket.BinaryMessage, messageContent)
		}
	}
}
