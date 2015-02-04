package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/justinsb/gova/log"
)

var flagListenAddr = flag.String("listen", ":80", "address for http server")

type httpHandler struct {
}

func (self *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	host := r.Host
	log.Debug("%v %v  %v", r.Method, host, urlPath)

	log.Debug("No mapping for %v", host)
	http.NotFound(w, r)
	return
}

func main() {
	flag.Parse()

	httpHandler := &httpHandler{}

	s := &http.Server{
		Addr:           *flagListenAddr,
		Handler:        httpHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Error("%v", s.ListenAndServe())
	os.Exit(1)
}
