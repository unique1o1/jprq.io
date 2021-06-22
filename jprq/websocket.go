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
	responseMessage, ok := <-requestMessage.ResponseChan
	if !ok {
		log.Println("closing responsechan")
		return
	}
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
				tunnel.requestChan <- RequestMessage{Status: -1}
				errClient <- err
				log.Println(err)
				break
			}
			log.Println(string(message))
			requestMessage := PackageWSRequest(message, msgType)
			select {
			case <-tunnel.requestChanCloseNotifier:
				log.Println("Exiting ...")
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
			_ = ws.WriteMessage(responseMessage.SocketMsgType, responseMessage.Body)
		}
		_ = ws.WriteMessage(websocket.CloseMessage, responseMessage.Body)

		log.Println("exiting as well")

	}()
	{
		var message string
		select {
		case err = <-errClient:
			{
				// close client side goroutine too
				rChan, ok := tunnel.requestsTracker.Load(rId)
				if ok {
					log.Println("closing resons")
					close(rChan.(chan ResponseMessage))
				}
				tunnel.requestsTracker.Delete(rId) // delete from map as well otherwise channel may be closed twice
			}
			message = "websocketproxy: Error when copying from client to backend: %v"
			log.Printf(message, err)
		case err = <-errBackend:
			message = "websocketproxy: Error when copying from backend to client: %v"
			log.Printf(message, err)
		}
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
		log.Println(msgType)
		if err != nil {
			log.Println(err)
			if websocket.IsCloseError(err, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
				log.Println("here")
				break
			}

			log.Println(err)
			//responseChan <- response // sending this will cause the goroutine to return so no need to close the channel

			break
		}
		response := ResponseMessage{}
		err = bson.Unmarshal(message, &response)
		if err != nil {
			log.Println("Failed to Unmarshal Websocket Message: ", string(message), err)
			continue
		}
		if response.Status == -1 { // for websockets
			r, ok := tunnel.requestsTracker.Load(response.RequestId)
			if !ok {
				continue
			}
			tunnel.requestsTracker.Delete(response.RequestId) // should delete chan from sync map otherwise it will be closed twice
			close(r.(chan ResponseMessage))
			continue
		}
		if response.Token != tunnel.token {
			log.Println("Authentication Failed: ", tunnel.host)
			continue
		}
		r, ok := tunnel.requestsTracker.Load(response.RequestId)
		if !ok {
			log.Println("Request Not Found", response.RequestId)
			continue
		}
		responseChan := r.(chan ResponseMessage)
		tunnel.numOfReqServed++
		responseChan <- response
		log.Println("end of forloop")
	}
	log.Println("end of func web")

}
