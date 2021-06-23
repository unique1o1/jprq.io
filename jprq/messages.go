package jprq

import (
	"bytes"
	"github.com/gofrs/uuid"
	"io"
	"io/ioutil"
	"net/http"
)

type ErrorMessage struct {
	Error string `bson:"error"`
}

type TunnelMessage struct {
	Host  string `bson:"host"`
	Token string `bson:"token"`
}

type RequestMessage struct {
	Status        int                  `bson:"status,omitempty"`
	SocketMsgType int                  `bson:"socket_msg_type,omitempty"`
	ID            uuid.UUID            `bson:"id,omitempty"`
	Method        string               `bson:"method,omitempty"`
	URL           string               `bson:"url,omitempty"`
	Body          []byte               `bson:"body,omitempty"`
	Header        http.Header          `bson:"header,omitempty"`
	Cookie        []*http.Cookie       `bson:"cookie,omitempty"`
	ResponseChan  chan ResponseMessage `bson:"-"`
}

type ResponseMessage struct {
	RequestId     uuid.UUID      `bson:"request_id,omitempty"`
	SocketMsgType int            `bson:"socket_msg_type,omitempty"`
	Token         string         `bson:"token,omitempty"`
	Body          []byte         `bson:"body,omitempty"`
	Status        int            `bson:"status,omitempty"`
	Header        http.Header    `bson:"header,omitempty"`
	Cookie        []*http.Cookie `bson:"cookie,omitempty"`
}

func PackageHttpRequest(httpRequest *http.Request) RequestMessage {
	requestMessage := RequestMessage{}
	requestMessage.ID, _ = uuid.NewV4()
	requestMessage.Method = httpRequest.Method
	requestMessage.URL = httpRequest.URL.RequestURI()
	requestMessage.Cookie = httpRequest.Cookies()
	requestMessage.Header = httpRequest.Header

	if httpRequest.Body != nil {
		body, _ := ioutil.ReadAll(httpRequest.Body)
		requestMessage.Body = body
	}

	//requestMessage.Header = make(map[string][]string)

	requestMessage.ResponseChan = make(chan ResponseMessage)

	return requestMessage
}

func (responseMessage ResponseMessage) WriteToHttpResponse(writer http.ResponseWriter, r *http.Request) {
	// Set CORS Headers
	writer.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	writer.Header().Set("Access-Control-Allow-Headers", "*")
	writer.Header().Set("Access-Control-Max-Age", "86400")
	for name, value := range responseMessage.Header {
		for _, v := range value {
			writer.Header().Add(name, v)
		}
	}
	for _, cookie := range responseMessage.Cookie {
		cookie.Path = "/"
		http.SetCookie(writer, cookie)
	}

	if 100 > responseMessage.Status || responseMessage.Status > 600 {
		responseMessage.Status = http.StatusInternalServerError
	}
	writer.WriteHeader(responseMessage.Status)

	decoded := responseMessage.Body
	io.Copy(writer, bytes.NewBuffer(decoded))
}
