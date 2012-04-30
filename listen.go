package main

import "encoding/gob"
import "log"
import "os"
import "path"
import "net/http"
import "html/template"
import "sync"

var dumpchan chan string 
var listen struct {
	M map[string]bool
	Count int
}
var listenlock sync.RWMutex

func ListenProc() {
	listenlock.Lock()
	g, err := os.Open(path.Join(MapGobPath, "listen"))
	if err == nil {
		gd := gob.NewDecoder(g)
		err = gd.Decode(&listen)
		if err != nil || listen.M == nil {
			listen.M = make(map[string]bool)
			if err != nil {
				log.Println("error reading stats", err)
			}
		}
		g.Close()
		log.Println("loaded", len(listen.M),"listen stats. next file is #",listen.Count)
	} else {
		listen.M = make(map[string]bool)
		log.Println("couldn't load listen stats", err)
	}
	listenlock.Unlock()
	listenlock.RLock()
	d, err := os.OpenFile(dumppath, os.O_RDONLY, 0600)
	if err != nil {
		log.Fatalln("dumppath should have been created by now", err)
	}
	names, err := d.Readdirnames(-1)
	if err != nil {
		log.Println("couldn't read all names from dumppath", err)
	}
	for _, name := range names {
		fullname := path.Join(dumppath, name)
		if !listen.M[fullname] {
			err = os.Remove(fullname)
			if err != nil {
				log.Println("couldn't remove", fullname, err)
			}
		}
	}
	for d := range dumpchan {
		log.Println("recvd", d)
		listenlock.Lock()
		listen.M[d] = true
		g, err := os.Create(path.Join(MapGobPath, "listen"))
		if err == nil {
			ge := gob.NewEncoder(g)
			err = ge.Encode(&listen)
			if err != nil {
				log.Println("error writing stats", err)
			}
			g.Close()
		} else {
			log.Println("couldn't write listen stats", err)
		}
		listenlock.Unlock()
	}
}

func ListenHandler(resp http.ResponseWriter, req *http.Request) {
	listenlock.RLock()
	defer listenlock.RUnlock()
	// if no arg, serve template with listen
	f := req.FormValue("file")
	if f == "" {
		t, err := template.ParseFiles(path.Join(templateRoot,"listen.html"))
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(resp, "Template Not Found", 404)
			} else {
				http.Error(resp, err.Error(), 500)
			}
			return
		}
		err = t.Execute(resp, listen.M)
		if err != nil {
			http.Error(resp, err.Error(), 500)
		}
	} else if listen.M[f] {
		http.ServeFile(resp, req, path.Join(dumppath, path.Base(f)))
	} else {
		http.Error(resp, "File's not ready!", 404)
	}
}

func init() {
	dumpchan = make(chan string,1)
	gofuncs = append(gofuncs, ListenProc)
	http.HandleFunc("/listen", ListenHandler)
}