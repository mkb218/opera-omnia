package main

import "math"
import "log"
import "os/exec"
import "github.com/mkb218/egonest/src/echonest"

type Request struct {
	Data []byte
	Filetype string
	Id string
	Corpus bool
	Resynth bool
}

type Segment {
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

const TimbreWeight = 1
const PitchWeight = 1
const LoudStartWeight = 1
const LoudMaxWeight = 1
const DurationWeight = 1
const BeatWeight = 1
const ConfidenceWeight = 1

func (s *Segment) Distance(s0 *Segment) float64 {
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

function pitch_distance(v1, v2 *Segment) {
    var sum float64

    for i := 0; i < 12; i++ {
        var delta = v2.Pitch[i] - v1.Pitch[i];
        sum += delta * delta;
    }
    return math.Sqrt(sum);
}
type AnalyzeResult struct {
	Id string
	Segments []Segment
	HasFiles bool
}

func LibManager struct {
	SegmentDir string
	OutputDir string
	ReqChan chan Request
	ResynthChan chan string
	AnalyzeChan chan []byte
	AnalyzeReturnChan chan AnalyzeResult
	AnalyzeRequestChan chan string
	AnalysisMap map[string][]Segment
	AllSegments []Segment // only ones with file backing
}

func InitLibManager(SegmentDir, OutputDir, EchonestKey string) (l *LibManager) {
	l = &LibManager{SegmentDir, OutputDir}
	l.ReqChan = make(chan Request, 10)
	l.ResynthChan = make(chan string, 10)
	l.ResynthQueue = make([]string,0)
	l.AnalyzeChan = make(chan Request, 1000)
	l.AnalyzeReturnChan = make(chan AnalyzeResult, 10)
	l.AnalyzeRequestChan = make(chan string)
	l.AnalyzeResponseChan = make(chan []Segment)
	l.AnalysisMap = make(map[string][]Segment)
	l.AllSegments = make([]Segment)
	go l.DrainReq()
	go l.DrainAnalyze(EchonestKey)
	go l.DrainResynth()
	go l.DrainAnalyzeReturn()
	return
}

func (l *LibManager) DrainReq() {
	for {
		r := <- l.ReqChan:
		if len(r.Id) > 0 {
			l.ResynthChan <- r.Id
		}
		if len(r.Data) > 0 {
			l.AnalyzeChan <- r
		}
	}
}

func (l *LibManager) DrainAnalyzeReturn() {
	for {
		select {
		case r := <- l.AnalyzeReturnChan:
			l.AnalysisMap[r.Id] = r.Segments
			if r.HasFiles {
				l.AllSegments = append(l.AllSegments, r.Segments)
			}
		case id := <- l.AnalyzeRequestChan:
			l.AnalyzeResponseChan <- l.AnalysisMap[id]
		}
	}
}

func getStart(i interface{}) float64 {
	b := i.(map[string]interface{})
	return b["start"].(float64)
}

func (l *LibManager) DrainAnalyze(EchonestKey string) {
	var e Echonest
	e.Key = EchonestKey
	for {
		r := <- l.AnalyzeChan
		id, a, err := e.Upload(r.Filetype, r.Data)
		if err != nil {
			log.Print("echonest error:",err)
			continue
		}
		at, ok := a.(map[string]interface{})
		if !ok {
			log.Print(id, "analyze response wasn't json map")
			continue
		}
		t, ok := at["track"].(map[string]interface{})
		if !ok {
			log.Print(id, "track data missing or not assertable as map[string]interface{}")
			continue
		}
		beats, ok := t["beats"].([]interface{})
		if !ok {
			log.Print(id, "beat data missing?")
			continue
		}
		segments, ok := t["segments"].([]interface{})
		if !ok {
			log.Print(id, "segment data missing?")
			continue
		}
		result := AnalyzeResult{id, make([]Segment, len(segments)}
		b0 := getStart(beats[0])
		b1 := getStart(beats[1])
		for index, seg := range segments {
			s := seg.(map[string]interface{})
			result[index].Id = id
			result[index].Index = index
			result[index].Start = s["start"].float64
			result[index].Duration = s["duration"].float64
			result[index].LoudnessStart = s["loudness_start"].float64
			result[index].LoudnessMax  = s["loudness_max"].float64
			result[index].Confidence  = s["confidence"].float64
			if result[index].Start > b1 {
				beats = beats[1:]
				b0 = b1
				b1 = getStart(beats[1])
			}
			result[index].BeatDistance = result[index].Start - b0
			copy(result[index].Pitches[:], s["pitches"].([]float64))
			copy(result[index].Timbre[:], s["timbre"].([]float64))
			// assign files later?
		}
		
			
		
	}