package main

import "log"
import "net/http"
import "html/template"
import "flag"

func IndexHandler(templateroot string) func(resp http.ResponseWriter, req *http.Request), error {
	
}

type StorageMgr struct {
	
}

func main() {
	template := flag.String("template", "templates", "path to templates")
	storagedir := flag.String("storagedir", "segments", "path where segments DB is stored")
	echonestkey := flag.String("echonestkey", "", "Echo Nest API key")
	serveraddress := flag.String("serveraddress", "localhost", "Streaming server address")
	serverport := flag.Int("serverport", 8000, "Streaming server port")
	// TODO: whatever else libshout needs
	flag.Parse()
	ihandler, err := IndexHandler(*template)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("initing storage manager")
	storageMgr := initStorageMgr(*storagedir)
	http.HandleFunc("/", ihandler) // serves upload template
	http.HandleFunc("/upload", UploadHandler(storageMgr))
	go Streamer(storageMgr)
	http.HandleFunc("/stream", StreamHandler(serveraddress, serverport)) // stream handler will basically redirect to the icecast server
	http.Handle("/", r)
	http.ListenAndServe(":3920", nil)
}