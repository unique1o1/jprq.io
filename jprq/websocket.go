package jprq

import (
	"errors"
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

func (j *Jprq) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	tunnel, err := j.GetTunnelByHost(r.Host)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}
	requestMessage := PackageHttpRequest(r)
	rId := requestMessage.ID

	tunnel.requestsTracker.Store(requestMessage.ID, requestMessage.ResponseChan)
	tunnel.requestChan <- requestMessage
	responseMessage := <-requestMessage.ResponseChan
	log.Println(responseMessage.Header)
	// Only pass those headers to the upgrader.
	upgradeHeader := http.Header{}
	{
		if hdr := responseMessage.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
			upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
		}
		if hdr := responseMessage.Header.Get("Set-Cookie"); hdr != "" {
			upgradeHeader.Set("Set-Cookie", hdr)
		}
	}
	ws, err := upgrader.Upgrade(w, r, upgradeHeader)
	if err != nil {
		log.Println("Error occurred when creating socket: ", err)
		return
	}
	defer ws.Close()

	PackageWSRequest := func(body []byte, msgType int) RequestMessage {
		requestMessage := RequestMessage{}
		requestMessage.SocketMsgType = msgType
		requestMessage.ID = rId
		requestMessage.Body = body
		return requestMessage
	}

	errClient := make(chan error, 1)
	errBackend := make(chan error, 1)
	go func() {
		for {
			msgType, message, err := ws.ReadMessage()
			if err != nil {
				errClient <- err
				log.Println(err)
				break
			}
			log.Println(string(message))
			requestMessage := PackageWSRequest(message, msgType)
			select {
			case <-tunnel.requestChanCloseNotifier:
				log.Println("exiting...")
				break
			default:
				log.Println("in default")
				tunnel.requestChan <- requestMessage

			}

		}
		log.Println("exiting as well")

	}()
	go func() {
		r, ok := tunnel.requestsTracker.Load(rId)
		if !ok {
			return
		}
		for responseMessage = range r.(chan ResponseMessage) {
			if responseMessage.SocketMsgType == websocket.CloseMessage {
				ws.WriteMessage(websocket.CloseMessage, responseMessage.Body)
				errBackend <- errors.New(string(responseMessage.Body))
				return
			}
			ws.WriteMessage(responseMessage.SocketMsgType, responseMessage.Body)
		}
		log.Println("exiting as well")

	}()
	{
		var message string
		select {
		case err = <-errClient:
			message = "websocketproxy: Error when copying from client to backend: %v"
			log.Printf(message, err)
		case err = <-errBackend:
			message = "websocketproxy: Error when copying from backend to client: %v"
			log.Printf(message, err)
		}
	}
	{
		rChan, ok := tunnel.requestsTracker.Load(rId)
		if ok {
			log.Println("closing resons")
			close(rChan.(chan ResponseMessage))
		}
		tunnel.requestsTracker.Delete(rId)
	}

}
func (j *Jprq) ClientWebsocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("New socket connection request....")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error occurred when creating socket: ", err)
		return
	}
	defer ws.Close()

	query := r.URL.Query()
	usernames := query["username"]
	if len(usernames) != 1 {
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
		msgType, message, err := ws.ReadMessage()
		if err != nil {
			log.Println(msgType)
			if e, ok := err.(*websocket.CloseError); ok {
				if e.Code != websocket.CloseNoStatusReceived {
					m := websocket.FormatCloseMessage(e.Code, e.Text)
					log.Println(string(m))
				} else {
					response := ResponseMessage{}
					err = bson.Unmarshal(message, &response)
					r, _ := tunnel.requestsTracker.Load(response.RequestId)
					responseChan := r.(chan ResponseMessage)
					responseChan <- response // sending this will cause the goroutine to return so no need to close the channel
				}
				log.Println(e)

				continue
			}
			log.Println(err)
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
		r, ok := tunnel.requestsTracker.Load(response.RequestId)
		responseChan := r.(chan ResponseMessage)
		if !ok {
			log.Println("Request Not Found", response.RequestId)
			continue
		}
		tunnel.numOfReqServed++
		responseChan <- response
	}
}
