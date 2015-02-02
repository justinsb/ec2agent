package main

import (
	"flag"
	"log"
	"os"
	"time"
	"net/http"
	"strings"
	"path"
	"net/url"
	"fmt"
	"io"
)

var flagListenAddr = flag.String("listen", "169.254.169.254:80", "address for http server")
var flagBasedir = flag.String("basedir", "http", "base directory for http content to serve")

type httpHandler struct {
	baseFs http.FileSystem
	basePath string
}

func (self *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/"+urlPath
		r.URL.Path = urlPath
	}
	log.Println(r.Method, urlPath)
	urlPath = path.Clean(urlPath)

	if strings.Contains(urlPath, "..") {
		http.NotFound(w, r)
		return
	}

	clientIp := r.RemoteAddr
	colonIndex := strings.Index(clientIp, ":")
	if colonIndex != -1 {
		clientIp = clientIp[:colonIndex]
	}
	name := clientIp + "/" + urlPath
	log.Println("Mapping to filepath", name)


	f, err := self.baseFs.Open(name)
	if err != nil {
		// TODO expose actual error?
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	d, err1 := f.Stat()
	if err1 != nil {
		// TODO expose actual error?
		http.NotFound(w, r)
		return
	}

	if d.IsDir() {
		if strings.HasSuffix(name, "/public-keys") {
			log.Println("Faking ouptut for public-keys")
			io.WriteString(w, "0=fathomdb")
			return
		}
		self.dirList(w, f)
		return
	}

	osFile := f.(*os.File)
	log.Println("Will server file", osFile.Name())

	http.ServeFile(w, r, osFile.Name())
}

func (self *httpHandler) dirList(w http.ResponseWriter, f http.File) {
	for {
		dirs, err := f.Readdir(100)
		if err != nil || len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			url := url.URL{Path: name}
			fmt.Fprintf(w, "%s\n", url.String())
		}
	}
}

func main() {
	flag.Parse()

	httpHandler := &httpHandler{}
	httpHandler.baseFs = http.Dir(*flagBasedir)

	s := &http.Server{
		Addr:          *flagListenAddr,
		Handler:        httpHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
