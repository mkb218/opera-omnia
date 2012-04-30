package main

import "bytes"
import "fmt"
import "sort"
import "math"
import "net/http"
import "log"
import "path"
import "html/template"
import "io"
import "os/exec"
import "strconv"
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
const PitchWeight = 100
const LoudStartWeight = 1
const LoudMaxWeight = 1
const DurationWeight = 1
const BeatWeight = 1
const ConfidenceWeight = 1
const IdentityWeight = 10000

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
	if s.Id == s0.Id {
		distance += IdentityWeight
	}
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
	return s.slice[i].Distance < s.slice[j].Distance
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
	// f, _ := os.OpenFile("cmds", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	// defer f.Close()

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
	
	soxp, err := exec.LookPath("sox")
	if err != nil {
		log.Panicln("couldn't find sox on PATH!", err)
	}
	// ideally we could keep reading segments until the read buffer is full
	if len(b) > 0 && len(a.segments) > 0 {
		// if b is still not empty, open next Segment's file, read all contents and copy as much to b as will fit
		s := a.segments[0]
		a.segments = a.segments[1:]
		
		args := []string{"-b", "16", "-c", "2", "-e", "signed-integer", "-t", "raw", "-r", strconv.Itoa(samplerate), "-B", s.File,  "-b", "16", "-c", "2", "-e", "signed-integer", "-t", "raw", "-r", strconv.Itoa(samplerate), "-B", "-", "stretch", strconv.FormatFloat(s.RootDuration/s.Duration, 'g', -1, 64)}
		rootavg := (s.RootLoudnessMax - s.RootLoudnessStart) / 2
		avg := (s.RootLoudnessMax - s.RootLoudnessStart) / 2
		if rootavg > avg {
			args = append(args, "gain", fmt.Sprintf("%f", rootavg - avg))
		} 
		
		sox := exec.Command(soxp, args...)
		// fmt.Fprintln(f, "play", args)
		ebuf := new(bytes.Buffer)
		sox.Stderr = ebuf
		var buf []byte
		buf, err = sox.Output()
		if err != nil {
			log.Println("sox failed", args, err)
			log.Println(string(ebuf.Bytes()))
			return
		}
		copy(b, buf)
		if len(buf) > len(b) {
			n += len(b)
			a.leftover = buf[len(b):]
		} else {
			n += len(buf)
		}
		
	}
		
	// copy rest to leftover
	// if no more segments, return EOF
	if len(a.segments) == 0 && len(a.leftover) == 0 {
		err = io.EOF
	}
	// log.Println("wrote", n, "bytes")
	return
}

func RequestProc() {
	log.Println("starting RequestProc")
	e := echonest.New()
	if echonestkey != "" {
		e.Key = echonestkey
	}
	for r := range RequestQueue {
		log.Println("got request for ID", r)
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
		go func() {
			var ar AudioRequest
			allSegsLock.Lock()
			defer allSegsLock.Unlock()
			expectedlen := float64(0)
			outlen := float64(0)
			for _, segment := range s.Segments {
				var ss SegSortSlice
//				m := make(map[SegmentID]bool)
/*				var pitches = []int{12,12,12}
				// find highest three pitches in segment
				for note, p := range segment.Pitches[:] {
					if pitches[0] == 12 || p > segment.Pitches[pitches[0]] {
						pitches[2], pitches[1], pitches[0] = pitches[1], pitches[0], note
					} else if pitches[1] == 12 || p > segment.Pitches[pitches[1]] {
						pitches[2], pitches[1] = pitches[1], note
					} else if pitches[2] == 12 || p > segment.Pitches[pitches[2]] {
						pitches[2] = note
					}
				}

				for index, n := range pitches {
					i := 0
					for k := range m {
						m[k] = false
					}
					
					for _, v := range allSegs.Pitch[n].Segments[int(segment.Pitches[n] / allSegs.Pitch[n].Width)] {
						if _, ok := m[v]; (index == 0) || ok {
							i++
							m[v] = true
						}
					}
					if i <= 100 {
						break
					}
				}*/
				
				// ss.slice = make([]Segment, 0, len(m))
				for k/*, b*/ := range allSegs.Segs {
					// if b {
						ss.slice = append(ss.slice, allSegs.Segs[k])
						ss.slice[len(ss.slice)-1].Distance = Distance(&segment, &ss.slice[len(ss.slice)-1])
//					}
				}

				ss.root = segment
				sort.Sort(ss)
				if len(ss.slice) > 0 {
				var outs Segment = ss.slice[0]
				// var mindist float64 = -1
				// var distcount int
				// for _, b := range allSegs.Segs {
				// 	outs = b
				// 	if mindist < 0 {
				// 		mindist = Distance(&segment, &outs)
				// 		log.Println("m", mindist)
				// 	} else if distcount < 10 {
				// 		if d := Distance(&segment, &outs); d < mindist {
				// 			mindist = d
				// 			distcount++
				// 			log.Println(mindist)
				// 		}
				// 	} else {
				// 		break
				// 	}
				// }
				outs.RootDuration = segment.Duration
				outs.RootLoudnessMax = segment.LoudnessMax
				outs.RootLoudnessStart = segment.LoudnessStart
				outlen += segment.Duration
				ar.segments = append(ar.segments, outs)
				}
				expectedlen += segment.Duration
			}
			log.Println(expectedlen, outlen)
			ar.artist = s.Artist
			ar.title = s.Title
			AudioQueue <- ar
		} ()
		
	}
}

var allSegments []Segment


func init() {
	RequestQueue = make(chan string,1)
	AudioQueue = make(chan AudioRequest,1)
	gofuncs = append(gofuncs, RequestProc)
	http.HandleFunc("/request", RequestHandler)
}

func RequestHandler(resp http.ResponseWriter, req *http.Request) {
	log.Println("request id")
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