package main

import (
	"flag"
	"fmt"
	"github.com/azimjohn/jprq.io/jprq"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

var baseHost string

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.StringVar(&baseHost, "host", "yunik.com.np", "Base Host")
	flag.Parse()

	j := jprq.New(baseHost)
	r := mux.NewRouter()
	r.HandleFunc("/_ws/", j.JPRQClientWebsocketHandler)
	r.PathPrefix("/").HandlerFunc(j.RequestHandler)
	fmt.Println("Server is running on Port 4200")
	log.Fatal(http.ListenAndServe(":4200", r))
}
