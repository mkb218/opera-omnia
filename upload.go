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
import "time"
import "encoding/base64"
import "math/rand"
import "errors"
import "encoding/gob"
import "io"
import "github.com/mkb218/egonest/src/echonest"

func mapKey(in interface{}, k string) interface{} {
	if m, ok := in.(map[string]interface{}); ok {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

var tmpLock sync.Mutex
var tmpnamesize = base64.URLEncoding.EncodedLen(8)

func mktemp(prefix string) (*os.File, error) {
	tmpLock.Lock()
	defer tmpLock.Unlock()
	for {
		namen := rand.Int63()
		var b []byte
		for i := 0; i < 8; i++ {
			b = append(b, byte(namen>>uint(i*8) & 0xff))
		}
		c := make([]byte, tmpnamesize)
		base64.URLEncoding.Encode(c, b)
		p := path.Join(os.TempDir(), string(c))
		if f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666); err == nil {
			return f, nil
		} else {
			log.Println("couldn't create tmp file", namen, "reason",err)
			if !os.IsExist(err) {
				return nil, err
			}
		}
	}
	panic("unreachable")
}

type SegmentID struct {
	Id string
	Index int
}

var allSegsLock sync.RWMutex
var allSegs struct {
	Segs map[SegmentID]Segment
//	timbre [12]bucket
	// Pitch [12]Bucket
}

func initAllSegs() {
	allSegsLock.Lock()
	defer allSegsLock.Unlock()
	r, err := os.Open(path.Join(MapGobPath, "allsegs"))
	if err != nil {
		log.Println("error opening map gob file", err)
		allSegs.Segs = make(map[SegmentID]Segment)
		return
	}
	defer r.Close()
	g := gob.NewDecoder(r)
	err = g.Decode(&allSegs) 
	if err != nil {
		log.Println("error decoding allSegs", err)
	}
	for k, v := range allSegs.Segs {
		fi, err := os.Stat(v.File)
		if err != nil || fi.IsDir() {
			delete(allSegs.Segs, k)
		}
	}
	log.Println("decoded", len(allSegs.Segs), "segments")
}

func AddToAllSegs(in []Segment) {
	allSegsLock.Lock()
	defer allSegsLock.Unlock()
	
	for _, r := range in {
		allSegs.Segs[r.SegmentID] = r
		for i := 0; i < 12; i++ {
			// bucketnum := int(r.Timbre[i] / allSegs.timbre[i].width)
			// allSegs.timbre[i].Segments[bucketnum] = append(allSegs.timbre[i].Segments[bucketnum], r.SegmentID)
//			bucketnum := int(r.Pitches[i] / allSegs.Pitch[i].Width)
//			allSegs.Pitch[i].Segments[bucketnum] = append(allSegs.Pitch[i].Segments[bucketnum], r.SegmentID)
		}
	}
	// balanceAllBuckets()
	// dump to gobfile
	w, err := os.Create(path.Join(MapGobPath, "allsegs"))
	if err != nil {
		log.Println("error creating gobfile for allsegs", err)
		return
	}
	defer w.Close()
	g := gob.NewEncoder(w)
	err = g.Encode(&allSegs)
	if err != nil {
		log.Println("error writing gobfile for allsegs", err)
	}
}

type UploadRequest struct {
	Data []byte
	Filetype string
	Add bool
	Playback bool
	Fma_url string
}

func minOf(i, j int64) int64 {
	if i < j {
		return i
	}
	return j
}

func fminOf(i, j float64) float64 {
	if i < j {
		return i
	}
	return j
}

func fmaxOf(i, j float64) float64 {
	if i > j {
		return i
	}
	return j
}

var UploadChan chan UploadRequest
var MapGobPath string
var samplepath string
var echonestkey string

type md5toid struct {
	Ids map[md5sum]string
}

type md5sum [16]byte

var Md5toidLock sync.Mutex

var Md5toid md5toid = md5toid{Ids:make(map[md5sum]string)}

func InitIDForChecksum() {
	Md5toidLock.Lock()
	defer Md5toidLock.Unlock()
	r, err := os.Open(path.Join(MapGobPath, "md5map"))
	if err != nil {
		log.Println("error opening map gob file", err)
		return
	}
	defer r.Close()
	g := gob.NewDecoder(r)
	g.Decode(&Md5toid)
	log.Println("decoded", len(Md5toid.Ids), "md5s")
}

func GetIDForChecksum(m md5sum) (val string, ok bool) {
	Md5toidLock.Lock()
	defer Md5toidLock.Unlock()
	val, ok = Md5toid.Ids[m]
	return 
}

func AddIDForChecksum(m md5sum, id string) {
	Md5toidLock.Lock()
	defer Md5toidLock.Unlock()
	Md5toid.Ids[m] = id
	// dump to gobfile
	w, err := os.Create(path.Join(MapGobPath, "md5map"))
	if err != nil {
		log.Println("error creating gobfile for map", err)
		return
	}
	defer w.Close()
	g := gob.NewEncoder(w)
	err = g.Encode(&Md5toid)
	if err != nil {
		log.Println("error writing gobfile for md5toid", err)
	}
}

type Segment struct {
	SegmentID
	Start float64
	Duration float64
	LoudnessStart float64
	LoudnessMax float64
	Confidence float64
	BeatDistance float64
	Pitches [12]float64
	Timbre [12]float64
	File string
	RootLoudnessMax float64
	RootLoudnessStart float64
	RootDuration float64
	Distance float64
}

type Analysis struct {
	Artist, Title string
	Segments []Segment
	Last time.Time
}

type ID2AnalysisResults struct {
	Ids map[string]Analysis
}

var id2analysisLock sync.Mutex
var id2analysis = ID2AnalysisResults{Ids:make(map[string]Analysis)}

func InitSegmentsForChecksum() {
	id2analysisLock.Lock()
	defer id2analysisLock.Unlock()
	r, err := os.Open(path.Join(MapGobPath, "idamap"))
	if err != nil {
		log.Println("error opening idamap gob file", err)
		return
	}
	defer r.Close()
	g := gob.NewDecoder(r)
	g.Decode(&id2analysis)
	log.Println("decoded", len(id2analysis.Ids), "ids")
}

func GetSegmentsForID(id string) (s Analysis, ok bool) {
	id2analysisLock.Lock()
	defer id2analysisLock.Unlock()
	s, ok = id2analysis.Ids[id]
	if (s.Last.UnixNano() > 0) && (time.Now().Sub(s.Last) > (30 * 24 * time.Hour)) {
		log.Println("expired")
		delete(id2analysis.Ids,id)
		return Analysis{}, false
	}
	return
}

func SetSegmentsForID(id string, segments Analysis) {
	id2analysisLock.Lock()
	defer id2analysisLock.Unlock()
	id2analysis.Ids[id] = segments
	// write to gob
	w, err := os.Create(path.Join(MapGobPath, "idamap"))
	if err != nil {
		log.Println("error creating gobfile for idamap", err)
		return
	}
	defer w.Close()
	g := gob.NewEncoder(w)
	err = g.Encode(&id2analysis)
	if err != nil {
		log.Println("error writing gobfile for idamap", err)
	}
}

func DetailsForID(url, id string) (a Analysis, err error) {
	// log.Println("DetailsForID", id, url)
	response, err := http.Get(url)
	if err != nil {
		log.Println("couldn't get details for", id)
		return Analysis{}, err
	}
	// log.Println("got", response.ContentLength, "bytes")
	
	var details interface{}
	j := json.NewDecoder(response.Body)
	j.Decode(&details)
	response.Body.Close()
	artist := mapKey(mapKey(details, "meta"), "artist")
	title := mapKey(mapKey(details, "meta"), "title")
	switch t := artist.(type) {
	case string:
		a.Artist = t
		log.Println("artist", artist)
	// default:
		// log.Println("no artist available")
	}
	switch t := title.(type) {
	case string:
		a.Title = t
		log.Println("title", title)
	default:
		// log.Println("no title available")
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
	
	a.Segments = make([]Segment, len(jsegments))
	SEG: for index, is := range jsegments {
		s := is.(map[string]interface{})
		a.Segments[index].Id = id
		a.Segments[index].Index = index
		a.Segments[index].Start = s["start"].(float64)
		a.Segments[index].Duration = s["duration"].(float64)
		a.Segments[index].LoudnessStart = s["loudness_start"].(float64)
		a.Segments[index].LoudnessMax  = s["loudness_max"].(float64)
		a.Segments[index].Confidence  = s["confidence"].(float64)
		if a.Segments[index].Start > b1 {
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
		a.Segments[index].BeatDistance = a.Segments[index].Start - b0
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
			a.Segments[index].Pitches[i], ok = p[i].(float64)
			if !ok {
				log.Println("can't coerce p element to float64")
				continue SEG
			}
			a.Segments[index].Timbre[i], ok = t[i].(float64)
			if !ok {
				log.Println("can't coerce t element to float64")
				continue SEG
			}
		}
	}
	// log.Println("details OK")
	a.Last = time.Now()
	return a, nil
}


func UploadProc() {
	log.Println("starting upload proc")
	// read md5 to ID mapping
	// read ID to analysis mapping
	
	hasher := md5.New()
	e := echonest.New()
	if echonestkey != "" {
		e.Key = echonestkey
	}
	
	for r := range UploadChan {
		log.Println("got", len(r.Data), "bytes",r.Filetype,"add",r.Add, "playback", r.Playback)
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
			// log.Println("no id for md5", m)
			// if not upload it to analyzer.
			id, url, err = e.Upload(r.Filetype, r.Data)
			if err != nil {
				log.Println("error uploading track to EN", err)
				continue
			}
			// log.Println("got ID", id, "url", url, "err", err)
			
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
		
		go func() {
			// if it's marked "add" open data with sox sub process (for mp3, mp4, and m4a support) to get raw samples
			has := (a.Segments[0].File != "")
			if r.Add && !has {
				buf, err := openBuf(r.Data, r.Filetype)
				if err != nil {
					log.Println("couldn't get sox to run", err)
					return
				}
				// put raw samples into files
				for i := range a.Segments {
					filename := id + "_" + strconv.Itoa(a.Segments[i].Index)
					filename = path.Join(samplepath, filename)
					file, err := os.Create(filename)
					if err != nil {
						log.Println("couldn't open file", filename, err)
						continue
					}
					bytecount := int(a.Segments[i].Duration * float64(samplerate)) * 4 // 2 bytes per sample * 2 channels per frame
					// log.Println(a.segments[i].Duration, bytecount)
					_, err = file.Write(buf[:bytecount])
					if err != nil {
						log.Println("error writing sample", err)
					}
					file.Close()
					a.Segments[i].File = filename
				
				}
				// add to all segments
				// log.Println("adding to all segs")
				go func() { attributionChan <- en_tuple{id, playq{a.Artist, a.Title, r.Fma_url} } }()
				AddToAllSegs(a.Segments)
				// log.Println("done adding to all segs")
			}

			// if request is marked "playback" add the ID to the request queue
			if r.Playback {
				RequestQueue <- id
			}
		}()
	}
}

func openBuf(data []byte, filetype string) (obuf []byte, err error) {
	p, err := exec.LookPath("sox")
	if err != nil {
		log.Fatalln("no sox installed!", err)
	}
	var c *exec.Cmd
	var reader io.Reader
	switch filetype {
	case "mp3": fallthrough
	case "wav": fallthrough
	case "ogg": fallthrough
	case "au":
		reader = bytes.NewReader(data)
	case "mp4": fallthrough
	case "m4a":
		// else write to tmpfile, then invoke sox, defer deletion of tmpfile. weak.
		w, err := mktemp(filetype)
		if err != nil {
			log.Println("couldn't make tmp file", err)
			return nil, err
		}

		defer func() { w.Close(); os.Remove(w.Name()) }()

		var n int
		n, err = w.Write(data)
		if n != len(data) {
			log.Println("couldn't write all data to tmp file!")
		}
		if err != nil {
			log.Println("couldn't write to tmp file", w.Name(), err)
			return nil, err
		}
		w.Seek(0, os.SEEK_SET)
		reader = w
	default:
		return nil, errors.New("unrecognized filetype")
	}

	// use samplerate from args, 16 bits, big endian, two channels
	c = exec.Command(p, "-t", filetype, "-", "-b", "16", "-c", "2", "-e", "signed-integer", "-t", "raw", "-r", strconv.Itoa(samplerate), "-B", "-")
	c.Stdin = reader
	errbuf := new(bytes.Buffer)
	c.Stderr = errbuf
	obuf, err = c.Output()
	if err != nil {
		log.Println("error running sox", err, string(errbuf.Bytes()))
		return nil, err
	}
	return
}

var rander *rand.Rand

func init() {
	flag.StringVar(&MapGobPath, "mapgobpath", "./gobs", "")
	flag.StringVar(&samplepath, "samples", "./samples", "")
	flag.StringVar(&echonestkey, "echonestkey", "", "")
	
	UploadChan = make(chan UploadRequest,100)
	gofuncs = append(gofuncs, UploadProc)
	http.HandleFunc("/upload", UploadHandler)
	rander = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func UploadHandler(resp http.ResponseWriter, req *http.Request) {
	t, fail := template.ParseFiles(path.Join(templateRoot, "upload_fail.html"))
	if fail != nil {
		log.Println("couldn't load template " + fail.Error())
	}
	// log.Println(req.Header)
	log.Println("upload")
	add := (req.FormValue("add") == "on")
	playback := (req.FormValue("playback") == "on")
	filetype := req.FormValue("filetype")
	fma_url := req.FormValue("fma_url")
	log.Println(add, playback, filetype, fma_url)
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
	r := UploadRequest{make([]byte, req.ContentLength), filetype, add, playback, fma_url}
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

