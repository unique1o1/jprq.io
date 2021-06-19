package jprq

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/gosimple/slug"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (j Jprq) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("New socket connection request....")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error occurred when creating socket: ", err)
		return
	}
	defer ws.Close()

	query := r.URL.Query()
	usernames := query["username"]
	if len(usernames) != 1  {
		log.Println("Websocket Connection: Bad Request: ", query)
		return
	}

	subdomain := usernames[0]
	subdomain = slug.Make(subdomain)
	host := j.GetUnusedHost(fmt.Sprintf("%s.%s", subdomain, j.baseHost), subdomain)
	tunnel := j.AddTunnel(host, ws)
	defer j.DeleteTunnel(tunnel.host)

	message := TunnelMessage{tunnel.host, tunnel.token}
	messageContent, err := bson.Marshal(message)

	ws.WriteMessage(websocket.BinaryMessage, messageContent)

	go tunnel.DispatchRequests()

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("Connection broken.")
			break
		}

		response := ResponseMessage{}
		err = bson.Unmarshal(message, &response)
		if err != nil {
			log.Println("Failed to Unmarshal Websocket Message: ", string(message), err)
			continue
		}

		if response.Token != tunnel.token {
			log.Println("Authentication Failed: ", tunnel.host)
			continue
		}

		requestMessage, ok := tunnel.requests[response.RequestId]
		if !ok {
			log.Println("Request Not Found", response.RequestId)
			continue
		}
		tunnel.numOfReqServed++
		delete(tunnel.requests, requestMessage.ID)
		requestMessage.ResponseChan <- response
	}
}
