package main

import "bytes"
import "crypto/md5"
import "encoding/json"
import "flag"
import "fmt"
import "html/template"
import "log"
import "net/http"
import "os"
import "os/exec"
import "path"
import "strconv"
import "sync"
import "github.com/mkb218/egonest/src/echonest"

func mapKey(in interface{}, k string) interface{} {
	if m, ok := in.(map[string]interface{}); ok {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

type AllSegs struct {
	sync.Mutex
	segs []Segment
}

var allSegs AllSegs

func AddToAllSegs(in []Segment) {
	allSegs.Lock()
	allSegs.segs = append(allSegs.segs, in...)
	allSegs.Unlock()
}

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
var samplepath string
var echonestkey string

type md5toid struct {
	sync.Mutex
	ids map[md5sum]string
}

type md5sum [16]byte

var Md5toid md5toid = md5toid{ids:make(map[md5sum]string)}

func GetIDForChecksum(m md5sum) (val string, ok bool) {
	Md5toid.Lock()
	defer Md5toid.Unlock()
	val, ok = Md5toid.ids[m]
	return 
}

func AddIDForChecksum(m md5sum, id string) {
	Md5toid.Lock()
	defer Md5toid.Unlock()
	Md5toid.ids[m] = id
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
	RootLoudness float64
	RootDuration float64
}

type Analysis struct {
	artist, title string
	segments []Segment
}

type ID2AnalysisResults struct {
	sync.Mutex
	ids map[string]Analysis
}

var id2analysis = ID2AnalysisResults{ids:make(map[string]Analysis)}

func GetSegmentsForID(id string) (s Analysis, ok bool) {
	id2analysis.Lock()
	defer id2analysis.Unlock()
	s, ok = id2analysis.ids[id]
	return
}

func SetSegmentsForID(id string, segments Analysis) {
	id2analysis.Lock()
	defer id2analysis.Unlock()
	id2analysis.ids[id] = segments
	// write to gob
}

func DetailsForID(url, id string) (a Analysis, err error) {
	log.Println("DetailsForID", id, url)
	response, err := http.Get(url)
	if err != nil {
		log.Println("couldn't get details for ", id)
		return Analysis{}, err
	}
	log.Println("got", response.ContentLength, "bytes")
	
	var details interface{}
	j := json.NewDecoder(response.Body)
	j.Decode(&details)
	response.Body.Close()
	artist := mapKey(mapKey(details, "meta"), "artist")
	title := mapKey(mapKey(details, "meta"), "title")
	switch t := artist.(type) {
	case string:
		a.artist = t
		log.Println("artist", artist)
	default:
		log.Println("no artist available")
	}
	switch t := title.(type) {
	case string:
		a.title = t
		log.Println("title", title)
	default:
		log.Println("no title available")
	}
	jsegments, ok := mapKey(details, "segments").([]interface{})
	if !ok {
		log.Println("no segments available for", id)
		return Analysis{}, err
	}
	jbeats, ok := mapKey(details, "beats").([]interface{})
	if !ok {
		log.Println("no beats available for", id)
		return Analysis{}, err
	}
	
	jb0, ok := jbeats[0].(map[string]interface{})
	jb1, ok := jbeats[1].(map[string]interface{})
	jbeats = jbeats[2:]
	
	b0, ok := jb0["start"].(float64)
	if !ok {
		log.Println("no beat start time")
		b0 = 0
	}

	b1, ok := jb1["start"].(float64)
	if !ok {
		log.Println("no beat start time")
		b1 = 0
	}
	
	a.segments = make([]Segment, len(jsegments))
	SEG: for index, is := range jsegments {
		s := is.(map[string]interface{})
		a.segments[index].Id = id
		a.segments[index].Index = index
		a.segments[index].Start = s["start"].(float64)
		a.segments[index].Duration = s["duration"].(float64)
		a.segments[index].LoudnessStart = s["loudness_start"].(float64)
		a.segments[index].LoudnessMax  = s["loudness_max"].(float64)
		a.segments[index].Confidence  = s["confidence"].(float64)
		if a.segments[index].Start > b1 {
			b0 = b1
			if len(jbeats) > 0 {
				jb := jbeats[0]
				jbeats = jbeats[1:]
				b1, ok = mapKey(jb,"start").(float64)
				if !ok {
					b1 = 0
				}
			}
		}
		a.segments[index].BeatDistance = a.segments[index].Start - b0
		p, ok := s["pitches"].([]interface{})
		if !ok {
			log.Println("no pitch info")
			continue
		}
		
		t, ok := s["timbre"].([]interface{})
		if !ok {
			log.Println("no timbre info")
			continue
		}
		
		for i := 0; i < 12; i++ {
			a.segments[index].Pitches[i], ok = p[i].(float64)
			if !ok {
				log.Println("can't coerce p element to float64")
				continue SEG
			}
			a.segments[index].Timbre[i], ok = t[i].(float64)
			if !ok {
				log.Println("can't coerce t element to float64")
				continue SEG
			}
		}
	}
	log.Println("details OK")
	return a, nil
}


func UploadProc() {
	log.Print("starting upload proc")
	// read md5 to ID mapping
	// read ID to analysis mapping
	
	hasher := md5.New()
	e := echonest.New()
	if echonestkey != "" {
		e.Key = echonestkey
	}
	
	for r := range UploadChan {
		log.Print("got ", len(r.Data), " bytes ",r.Filetype," add ",r.Add, " playback ", r.Playback)
		// md5 data and see if we have analysis already
		var m md5sum
		hasher.Write(r.Data)
		copy(m[:], hasher.Sum(nil))
		hasher.Reset()
		var id string
		var ok bool
		var a Analysis
		var url string
		var err error
		if id, ok = GetIDForChecksum(m); !ok {
			log.Println("no id for md5", m)
			// if not upload it to analyzer.
			id, url, err = e.Upload(r.Filetype, r.Data)
			if err != nil {
				log.Println("error uploading track to EN", err)
				continue
			}
			log.Println("got ID", id, "url", url, "err", err)
			
			// update md5 to id mapping
			AddIDForChecksum(m, id)
		}

		// if it comes back with an ID that we have, then great! 
		// if not then fetch the detailed analysis
		// update id to analysis mapping
		if a, ok = GetSegmentsForID(id); !ok {
			a, err = DetailsForID(url, id)
			if err != nil {
				log.Println("error getting details from EN", err)
				continue
			}
			SetSegmentsForID(id, a)
		}
		
		// if it's marked "add" open data with sox sub process (for mp3, mp4, and m4a support) to get raw samples
		if r.Add {
			p, err := exec.LookPath("sox")
			if err != nil {
				log.Fatalln("no sox installed!", err)
			}
			c := exec.Command(p, "-t", r.Filetype, "-", "-b", "16", "-c", "2", "-e", "signed-integer", "-t", "raw", "-r", strconv.Itoa(samplerate), "-B", "-")
			c.Stdin = bytes.NewReader(r.Data)
			sbuf := new(bytes.Buffer)
			c.Stdout = sbuf
			errbuf := new(bytes.Buffer)
			c.Stderr = errbuf
			// use samplerate from args, 16 bits, big endian, two channels
			err = c.Start()
			if err != nil {
				log.Println("error starting sox", err)
				continue
			}
			err = c.Wait()
			if err != nil {
				log.Println("error running sox", err, string(errbuf.Bytes()))
				continue
			}
			// put raw samples into files
			for i := range a.segments {
				filename := id + "_" + strconv.Itoa(a.segments[i].Index)
				filename = path.Join(samplepath, filename)
				file, err := os.Create(filename)
				if err != nil {
					log.Println("couldn't open file", filename, err)
					continue
				}
				bytecount := int(a.segments[i].Duration * float64(samplerate)) * 4 // 2 bytes per sample * 2 channels per frame
				// log.Println(a.segments[i].Duration, bytecount)
				_, err = file.Write(sbuf.Next(bytecount))
				if err != nil {
					log.Println("error writing sample", err)
				}
				file.Close()
				a.segments[i].File = filename
				
			}
			// add to all segments
			log.Println("adding to all segs")
			AddToAllSegs(a.segments)
			log.Println("done adding to all segs")
		}

		// if request is marked "playback" add the ID to the request queue
		if r.Playback {
			go func() { RequestQueue <- id }()
		}

	}
}

func init() {
	flag.StringVar(&MapGobPath, "mapgobpath", "/Users/mkb/code/opera-omnia/gobs", "")
	flag.StringVar(&samplepath, "samples", "/Users/mkb/code/opera-omnia/samples", "")
	flag.StringVar(&echonestkey, "echonestkey", "", "")
	
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

