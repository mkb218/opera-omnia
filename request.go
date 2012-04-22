package main

import "fmt"
import "net/http"
import "log"
import "path"
import "html/template"

var RequestQueue chan string
var AudioQueue chan AudioRequest

type AudioRequest struct {
	artist, title string
	segments []Segment
	leftover []byte
}

func (a *AudioRequest) Read(b []byte) (n int, err error) {
	// if leftover is not empty, copy to b
	// if b is still not empty, open next Segment's file, read all contents and copy as much to b as will fit
	// copy rest to leftover
	// if no more segments, return EOF
}

func RequestProc() {
	log.Print("starting RequestProc")
	for r := range RequestQueue {
		log.Print("got request for ID", r)
		// if it's 32 chars long, check if it's in md5
		// if it's an md5, then look up md5 to id
		// if it's an id see if we have analysis
		// if we don't have analysis see if echonest does
		// once we get analysis, start grabbing samples
		// write to audio queue
		// AudioQueue <- ar
	}
}

func init() {
	RequestQueue = make(chan string)
	AudioQueue = make(chan AudioRequest)
	gofuncs = append(gofuncs, RequestProc)
	http.HandleFunc("/request", RequestHandler)
}

func RequestHandler(resp http.ResponseWriter, req *http.Request) {
	log.Print("request id")
	t, fail := template.ParseFiles(path.Join(templateRoot, "request_fail.html"))
	id := req.FormValue("id")
	if len(id) == 0 {
		if fail != nil {
			fmt.Fprintf(resp, "Not only are my HTML templates missing, but you didn't give me an ID!")
		} else {
			t.Execute(resp, "You didn't give me an ID to request.")
		}
		return
	}
	
	// why wait?
	go func() { RequestQueue <- id } ()
	t, fail = template.ParseFiles(path.Join(templateRoot, "request_success.html"))
	if fail != nil {
		fmt.Fprintf(resp, "Request was successful, but the HTML templates are missing!")
	} else {
		t.Execute(resp, nil)
	}
}