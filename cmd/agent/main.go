package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/justinsb/gova/joiner"
	"github.com/justinsb/gova/log"
	"github.com/justinsb/gova/match"
	"github.com/justinsb/gova/splitter"
)

var (
	validChars = match.AnyOf("abcdefghijklmnopqrstuvwxyz0123456789-_:")
)

var flagListenAddr = flag.String("listen", "169.254.169.254:80", "address for http server")
var flagBasedir = flag.String("basedir", "http", "base directory for http content to serve")

type httpHandler struct {
	baseFs   http.FileSystem
	basePath string
}

func (self *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
		r.URL.Path = urlPath
	}
	log.Debug("%v %v", r.Method, urlPath)
	urlPath = path.Clean(urlPath)

	tokens := splitter.On("/").OmitEmptyStrings().Split(urlPath)
	for _, token := range tokens {
		if !validChars.MatchesAllOf(token) {
			http.NotFound(w, r)
			return
		}
	}

	clientIp := r.RemoteAddr
	colonIndex := strings.Index(clientIp, ":")
	if colonIndex != -1 {
		clientIp = clientIp[:colonIndex]
	}

	if len(tokens) >= 2 {
		version := tokens[0]
		if version == "openstack" {
			// Not handled
		} else {
			// Assume EC2
			// XXX: Properly check the version?

			key := tokens[1]
			if key == "user-data" && len(tokens) == 2 {
				self.serveFile(w, r, clientIp, "ec2/user-data")
				return
			}

			if key == "meta-data" {
				// Public keys are specially stored
				// (it looks like the original intention was to support multiple key formats)
				if len(tokens) >= 3 && tokens[2] == "public-keys" {
					publicKeys, err := self.listFiles(clientIp, "ec2/meta-data/public-keys")
					if err != nil {
						// XXX: Validate was not-found?
						log.Warn("Error reading public keys", err)
						http.NotFound(w, r)
						return
					}
					if len(tokens) == 3 {
						for i, name := range publicKeys {
							fmt.Fprintf(w, "%v=%s\n", i, name)
						}
						return
					}
					if len(tokens) >= 4 {
						i, err := strconv.Atoi(tokens[3])
						if err != nil {
							http.NotFound(w, r)
							return
						}
						if i < 0 || i >= len(publicKeys) {
							http.NotFound(w, r)
							return
						}
						if len(tokens) == 4 {
							// List formats for public-key
							fmt.Fprintf(w, "openssh-key\n")
							return
						}
						if len(tokens) == 5 && tokens[4] == "openssh-key" {
							self.serveFile(w, r, clientIp, "ec2/meta-data/public-keys/"+publicKeys[i])
							return
						}
					}
					http.NotFound(w, r)
					return
				}

				path := "ec2/" + joiner.On("/").Join(tokens[1:])
				self.serveFile(w, r, clientIp, path)
				return
			}
		}
	}

	log.Debug("No mapping for %v", urlPath)
	http.NotFound(w, r)
	return
}

func (self *httpHandler) serveFile(w http.ResponseWriter, r *http.Request, clientIp string, path string) {
	name := clientIp + "/" + path

	f, err := self.baseFs.Open(name)
	if err != nil {
		// XXX: Validate that was not-found?
		log.Warn("Error opening file %v", name)
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	d, err1 := f.Stat()
	if err1 != nil {
		// XXX: expose actual error?
		log.Warn("Error stat-ing file %v", name)
		http.NotFound(w, r)
		return
	}

	if d.IsDir() {
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
		return
	}

	osFile := f.(*os.File)
	log.Debug("Will serve file", osFile.Name())

	http.ServeFile(w, r, osFile.Name())
}

func (self *httpHandler) listFiles(clientIp string, path string) ([]string, error) {
	name := clientIp + "/" + path

	f, err := self.baseFs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	keys := []string{}

	dirs, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		keys = append(keys, name)
	}

	// Should be sorted, but double-check
	sort.Strings(keys)

	return keys, nil
}

func main() {
	flag.Parse()

	httpHandler := &httpHandler{}
	httpHandler.baseFs = http.Dir(*flagBasedir)

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
