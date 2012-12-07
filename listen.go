package main

/*import "encoding/gob"
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
	log.Println("ListenProc starting in lock")
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
		// log.Println("loaded", len(listen.M),"listen stats. next file is #",listen.Count)
	} else {
		listen.M = make(map[string]bool)
		log.Println("couldn't load listen stats", err)
	}
	listenlock.Unlock()
	// log.Println("unlocked write lock ListenProc")
	listenlock.RLock()
	// log.Println("locked read lock ListenProc")
	d, err := os.OpenFile(dumppath, os.O_RDONLY, 0600)
	if err != nil {
		log.Fatalln("dumppath should have been created by now", err)
	}
	found := make(map[string]bool)
	names, err := d.Readdirnames(-1)
	if err != nil {
		log.Println("couldn't read all names from dumppath", err)
	}
	for _, name := range names {
		fullname := path.Join(dumppath, name)
		if !listen.M[name] {
			err = os.Remove(fullname)
			if err != nil {
				log.Println("couldn't remove", fullname, err)
			}
		} else {
			log.Println("found dump file",name)
			found[name] = true
		}
	}
	d.Close()
	for n := range listen.M {
		if !found[n] {
			log.Println("no file for",n,"deleting from map")
			delete(listen.M, n)
		} else {
			log.Println("found file for",n)
		}
	}
	listenlock.RUnlock()
	log.Println("unlocked read lock ListenProc")
	for d := range dumpchan {
		log.Println("recvd", d)
		func() {
			listenlock.Lock()
			defer listenlock.Unlock()
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
		}()
		log.Println("done with write lock ListenProc")
	}
}

type listendata struct {
	Avail map[string]bool
	Queue map[playq]bool
}

func ListenHandler(resp http.ResponseWriter, req *http.Request) {
	// log.Println("ListenHandler waiting")
	listenlock.RLock()
	defer listenlock.RUnlock()
	// log.Println("ListenHandler", *req)
	
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
			log.Println("template error", err)
			return
		}
		playqlock.RLock()
		defer playqlock.RUnlock()
		err = t.Execute(resp, listendata{listen.M, playqueue})
		if err != nil {
			http.Error(resp, err.Error(), 500)
			log.Println("listen template error", err)
		}
	} else if listen.M[f] {
		log.Println("serving file", f)
		http.ServeFile(resp, req, path.Join(dumppath, path.Base(f)))
	} else {
		log.Println("no file")
		http.Error(resp, "File's not ready!", 404)
	}
}

func init() {
	dumpchan = make(chan string,100)
	gofuncs = append(gofuncs, ListenProc)
	http.HandleFunc("/listen", ListenHandler)
}
*/