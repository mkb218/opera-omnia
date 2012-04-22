package main

import "fmt"
import "math"
import "net/http"
import "log"
import "path"
import "html/template"
import "io"
import "io/ioutil"
import "sort"
import "github.com/mkb218/egonest/src/echonest"

var RequestQueue chan string
var AudioQueue chan AudioRequest

type distIndex struct {
	Id string
	Index string
}

type SegSortSlice struct {
	root Segment
	slice []Segment
}

const TimbreWeight = 1
const PitchWeight = 1
const LoudStartWeight = 1
const LoudMaxWeight = 1
const DurationWeight = 1
const BeatWeight = 1
const ConfidenceWeight = 1

func Distance(s, s0 *Segment) float64 {
    var timbre = timbre_distance(s, s0)
    var pitch = pitch_distance(s, s0)
    var sloudStart = math.Abs(s.LoudnessStart - s0.LoudnessStart)
    var sloudMax = math.Abs(s.LoudnessMax - s0.LoudnessMax)
    var duration = math.Abs(s.Duration - s0.Duration)
    var confidence = math.Abs(s.Confidence - s0.Confidence)
    var bdist = math.Abs(s.BeatDistance - s0.BeatDistance)
    var distance = timbre * TimbreWeight + pitch * PitchWeight + 
        sloudStart * LoudStartWeight + sloudMax * LoudMaxWeight + 
        duration * DurationWeight + confidence * ConfidenceWeight + bdist * BeatWeight
    return distance
}

func timbre_distance(s, s0 *Segment) float64 {
	var sum float64
    //for (var i = 0; i < 4; i++) {
    for i := 0; i < 12; i++ {
        var delta = s0.Timbre[i] - s.Timbre[i]
        //var weight = 1.0 / ( i + 1.0);
        var weight = 1.0
        sum += delta * delta * weight;
    }

    return math.Sqrt(sum);
}

func pitch_distance(v1, v2 *Segment) float64 {
    var sum float64

    for i := 0; i < 12; i++ {
        var delta = v2.Pitches[i] - v1.Pitches[i];
        sum += delta * delta;
    }
    return math.Sqrt(sum);
}

func (s SegSortSlice) Len() int {
	return len(s.slice)	
}

func (s SegSortSlice) Less(i, j int) bool {
	idist := Distance(&s.slice[i], &s.root)
	jdist := Distance(&s.slice[j], &s.root)
	return idist < jdist
}

func (s SegSortSlice) Swap(i, j int) {
	s.slice[i], s.slice[j] = s.slice[j], s.slice[i]
}

type AudioRequest struct {
	artist, title string
	segments []Segment
	leftover []byte
}

func (a *AudioRequest) Read(b []byte) (n int, err error) {
	// if leftover is not empty, copy to b
	if len(a.leftover) > 0 {
		if len(a.leftover) > len(b) {
			copy(b, a.leftover[0:len(b)])
			a.leftover = a.leftover[len(b):]
			return len(b), nil
		} else {
			copy(b, a.leftover)
			n += len(a.leftover)
			b = b[n:]
			a.leftover = a.leftover[:0]
		}
	}
	
	if len(b) > 0 {
		// if b is still not empty, open next Segment's file, read all contents and copy as much to b as will fit
		s := a.segments[0]
		a.segments = a.segments[1:]
		var buf []byte
		buf, err = ioutil.ReadFile(s.File)
		if err != nil && err != io.EOF {
			return
		}
		copy(b, buf)
		a.leftover = buf[len(b):]
		
	}
		
	// copy rest to leftover
	// if no more segments, return EOF
	return
}

func RequestProc() {
	log.Print("starting RequestProc")
	e := echonest.New()
	if echonestkey != "" {
		e.Key = echonestkey
	}
	for r := range RequestQueue {
		log.Print("got request for ID", r)
		// see if we have analysis for this ID
		s, ok := GetSegmentsForID(r)
		if !ok {
			url, err := e.Analyze(r)
			// if we don't have analysis see if echonest does
			if err != nil {
				log.Println("error grabbing analysis from EN", err)
				continue
			}
			s, err = DetailsForID(url, r)
			if err != nil {
				log.Println("error grabbing analysis from EN", err)
				continue
			}
		}

		// once we get analysis, start grabbing samples
		var ar AudioRequest
		allSegs.Lock()
		for _, segment := range s.segments {
			// here is the slow part
			var ss SegSortSlice
			ss.root = segment
			ss.slice = allSegs.segs
			sort.Sort(ss)
			ar.segments = append(ar.segments, ss.slice[0])
		}
		allSegs.Unlock()
			
		
		// write to audio queue
		go func() { AudioQueue <- ar } ()
	}
}

var allSegments []Segment


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