package main

import "html/template"
import "sync"
import "net/http"
import "path"
import "log"
import "fmt"
import "flag"

type UploadRequest struct {
	Data []byte
	Filetype string
	Add bool
	Playback bool
}

func minOf(i, j int64) int64 {
	if i < j {
		return i
	}
	return j
}

var UploadChan chan UploadRequest
var MapGobPath string

type md5toid struct {
	sync.Mutex
	ids map[string]string
}

var Md5toid md5toid = md5toid{ids:make(map[string]string)}

func GetIDForChecksum(md5 string) string {
	Md5toid.Lock()
	defer Md5toid.Unlock()
	return Md5toid.ids[md5]
}

func AddIDForChecksum(md5, id string) {
	Md5toid.Lock()
	defer Md5toid.Unlock()
	Md5toid.ids[md5] = id
	// dump to gobfile
}

type Segment struct {
	Id string
	Index int
	Start float64
	Duration float64
	LoudnessStart float64
	LoudnessMax float64
	Confidence float64
	BeatDistance float64
	Pitches [12]float64
	Timbre [12]float64
	File string
}

type ID2AnalysisResults struct {
	sync.Mutex
	ids map[string][]Segment
}

var id2analysis = ID2AnalysisResults{ids:make(map[string][]Segment)}

func GetSegmentsForID(id string) []Segment {
	id2analysis.Lock()
	defer id2analysis.Unlock()
	return id2analysis.ids[id]
}

func SetSegmentsForID(id string, segments []Segment) {
	id2analysis.Lock()
	defer id2analysis.Unlock()
	id2analysis.ids[id] = segments
	// write to gob
}

func UploadProc() {
	log.Print("starting upload proc")
	// read md5 to ID mapping
	// read ID to analysis mapping
	for r := range UploadChan {
		log.Print("got ", len(r.Data), " bytes ",r.Filetype," add ",r.Add, " playback ", r.Playback)
		// md5 data and see if we have analysis already
		// if not upload it to analyzer.
		// if it comes back with an ID that we have, then great! update md5 to id mapping
		// if not then fetch the detailed analysis
		// update id to analysis mapping
		// these need to be backed up on disk
		// once we have analysis:
		// if it's marked "playback" add the ID to the request queue
		// go func() { 
		// if it's marked "add" open data with libsox or sox sub process (for mp3, mp4, and m4a support) to get raw samples
		// use samplerate from args, 16 bits, big endian, two channels
		// put raw samples into files
	}
}

func init() {
	flag.StringVar(&MapGobPath, "mapgobpath", "/Users/mkb/code/opera-omnia/gobs", "")
	UploadChan = make(chan UploadRequest)
	gofuncs = append(gofuncs, UploadProc)
	http.HandleFunc("/upload", UploadHandler)
}

func UploadHandler(resp http.ResponseWriter, req *http.Request) {
	t, fail := template.ParseFiles(path.Join(templateRoot, "upload_fail.html"))
	if fail != nil {
		log.Print("couldn't load template " + fail.Error())
	}
	log.Print(req.Header)
	log.Print("upload")
	add := (req.FormValue("add") == "on")
	playback := (req.FormValue("playback") == "on")
	filetype := req.FormValue("filetype")
	log.Print(add, playback, filetype)
	file, _, err := req.FormFile("filedata")
	if !add && !playback {
		if fail != nil {
			fmt.Fprintf(resp, "Not only are my HTML templates missing, but you didn't tell me to do anything!")
		} else {
			t.Execute(resp, "If I can't add it to the library, and you don't want to hear it, why'd you bother uploading?")
		}
		return
	}
	
	if len(filetype) == 0 {
		if fail != nil {
			fmt.Fprintf(resp, "Not only are my HTML templates missing, but you didn't give me a filetype!")
		} else {
			t.Execute(resp, "I need a filetype!")
		}
		return
	}
	
	if err != nil {
		if fail != nil {
			fmt.Fprintf(resp, "Not only are my HTML templates missing, but there was an error uploading the file! %v", err)
		} else {
			t.Execute(resp, "There was an error uploading the file: "+err.Error())
		}
		return
	}
	
	// build the request to go into the channel
	r := UploadRequest{make([]byte, req.ContentLength), filetype, add, playback}
	n, err := file.Read(r.Data)
	if err != nil {
		if fail != nil {
			fmt.Fprintf(resp, "Not only are my HTML templates missing, but there was an error uploading the file! %v", err)
		} else {
			t.Execute(resp, "There was an error uploading the file: "+err.Error())
		}
		return
	}
		
	if n < len(r.Data) {
		r.Data = r.Data[0:n]
	}
	
	// why wait?
	go func () { UploadChan <- r } ()
	
	t, err = template.ParseFiles(path.Join(templateRoot, "upload_success.html"))
	t.Execute(resp, playback)
}

