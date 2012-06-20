package main

import "log"
import "os"
import "html/template"
import "path"
import "net/http"
// import "github.com/mkb218/egonest/src/echonest"
import "sync"
// import "encoding/json"
import "encoding/gob"

// type Fma struct {
// 	Track_url string `json:"track_url"`
// 	Track_title string `json:"track_title"`
// 	Artist_name string `json:"artist_name"`
// }
var en_datalock sync.RWMutex
var en_data = make(map[string]playq)
type en_tuple struct {
	string
	playq
}

var attributionChan = make(chan en_tuple, 100)
var startChan = make(chan bool, 1)

func AttributionHandler(resp http.ResponseWriter, req *http.Request) {
	<- startChan
	func() {
		// drain channel
		OUTER: for {
			select {
			case a := <- attributionChan:
				en_datalock.Lock()
				en_data[a.string] = a.playq
				en_datalock.Unlock()
			default:
				break OUTER
			}
		}
		en_datalock.RLock()
		defer en_datalock.RUnlock()
		// save gob
		g, err := os.Create(path.Join(MapGobPath, "attr"))
		if err != nil {
			log.Println("couldn't open en_data attributions")
		} else {
			ge := gob.NewEncoder(g)
			err = ge.Encode(en_data)
			if err != nil {
				log.Println("couldn't encode en_data", err)
			}
			g.Close()
		}
	}()
	
	en_datalock.RLock()
	defer en_datalock.RUnlock()
	
	t, err := template.ParseFiles(path.Join(templateRoot,"attribution.html"))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(resp, "Template Not Found", 404)
		} else {
			http.Error(resp, err.Error(), 500)
		}
		log.Println("template error", err)
		return
	}
	
	err = t.Execute(resp, en_data)
	if err != nil {
		http.Error(resp, err.Error(), 500)
		log.Println("attribution template error", err)
	}
	
}

func AttributionStart() {
	en_datalock.Lock()
	defer en_datalock.Unlock()
	g, err := os.Open(path.Join(MapGobPath, "attr"))
	if err != nil {
		log.Println("couldn't open en_data attributions")
	} else {
		gd := gob.NewDecoder(g)
		err = gd.Decode(&en_data)
		if err != nil {
			log.Println("couldn't decode en_data", err)
		}
		g.Close()
	}
	startChan <- true
	close(startChan)
}

func init() {
    http.HandleFunc("/attributions", AttributionHandler)
	// read gob with en data
	gofuncs = append(gofuncs, AttributionStart)
}