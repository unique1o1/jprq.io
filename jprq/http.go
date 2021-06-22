package jprq

import (
	"net/http"
)

func (j *Jprq) httpHandler(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	tunnel, err := j.GetTunnelByHost(host)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}

	requestMessage := PackageHttpRequest(r)
	tunnel.requestsTracker.Store(requestMessage.ID, requestMessage.ResponseChan)

	tunnel.requestChan <- requestMessage
	responseMessage, ok := <-requestMessage.ResponseChan
	tunnel.requestsTracker.Delete(requestMessage.ID)
	if !ok {
		w.WriteHeader(404)
		return
	}
	responseMessage.WriteToHttpResponse(w)

}

func (j *Jprq) RequestHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Upgrade") == "websocket" {
		j.WebsocketHandler(writer, request)
	} else {
		j.httpHandler(writer, request)

	}
}
