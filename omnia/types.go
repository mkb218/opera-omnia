package omnia

import "fmt"
import "encoding/json"
import "log"
import "sync"

// this is a raw audio file that has been uploaded. use libsox to determine its type and re-encode
// name comes from the form header
type AudioFile struct {
	Data []byte
	Name string
	Corpus bool
	Request bool
	InfoUrl string
}

func (af AudioFile) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// an audio file not (by itself) bound for the corpus. This can be used to create audio from the corpus
type PostAudioFile struct {
	TrackId string
	InfoUrl string
	Tatums []Tatum
}

func (paf PostAudioFile) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// an audio file bound for (or pulled out of) the corpus
type CorpusAudioFile struct {
	TrackId string
	InfoUrl string
	Tatums []AudioTatum
}

func (caf CorpusAudioFile) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// a tatum. This does not, by itself, belong in the corpus.
type Tatum struct {
	Chroma, Timbre []float64
	LoudnessMax float64
	Tstart, Tend float64
}

// an audiotatum. this belongs in the corpus.
// data is mp3-encoded, 64kbps, mono
type AudioTatum struct {
	Tatum
	Data []byte
	TrackId string
}

type ClosedError struct {
	*Queue
}

func (c ClosedError) Error() string {
	return fmt.Sprintf("%p already closed", c.Queue)
}

type inq struct {
	m json.Marshaler
	c chan struct{}
}

// A Queue is like a buffered channel, with no maximum size
type Queue struct {
	list []json.Marshaler
	pop chan json.Marshaler
	push chan inq
	length chan int
	closed bool
	sync.RWMutex
}

func init() {
	log.SetFlags(log.LstdFlags|log.Lshortfile)
}

func (a *Queue) appendToList(in inq) {
	a.list = append(a.list, in.m)
	log.Println("appended", a.list)
	if in.c != nil {
		in.c <- struct{}{}
		log.Println("signaled")
		close(in.c)
	}
}

func NewQueue(bufsize int) *Queue {
	a := new(Queue)
	a.push = make(chan inq, bufsize)
	a.pop = make(chan json.Marshaler, bufsize)
	go func() {
		for {
			in, ok := <-a.push
			// log.Println("in",in,"ok",ok)
			if ok {
				a.appendToList(in)
			} else {
				goto DRAIN
			}
			for len(a.list) > 0 {
				select {
				case in, ok := <- a.push:
					if ok {
						a.appendToList(in)
					} else {
						goto DRAIN
					}
				case a.pop <- a.list[0]:
					a.list = a.list[1:]
				}
			}
		}
		DRAIN: for len(a.list) > 0 {
			a.pop <- a.list[0]
			a.list = a.list[1:]
		}
		close(a.pop)
	}()
	return a
}

func (a *Queue) Pop() (i json.Marshaler, ok bool) {
	a.Lock()
	defer a.Unlock()
	i, ok = <- a.pop
	return
}

func (a *Queue) Push(i json.Marshaler) error {
	a.Lock()
	defer a.Unlock()
	// log.Println("push", i)
	return a.pushBase(inq{i, nil})
}

func (a *Queue) PushWait(i json.Marshaler) error {
	a.Lock()
	defer a.Unlock()
	c := make(chan struct{})
	defer func(){ <- c }()
	return a.pushBase(inq{i, c})
}

func (a *Queue) pushBase(i inq) error {
	if a.closed {
		return ClosedError{a}
	}
	a.push <- i
	log.Println("pushed", i)
	return nil
}

func (a *Queue) MarshalJSON() ([]byte, error) {
	a.RLock()
	defer a.RUnlock()
	out := []byte{'['}
	comma := ""
	for _, h := range a.list {
		out = append(out, []byte(comma)...)
		b, err := h.MarshalJSON()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
		comma = ","
	}
	out = append(out, ']')
	return out, nil
}

func (a *Queue) Stop() {
	if !a.closed {
		a.closed = true
		close(a.push)
	}
}

func (a *Queue) Len() int {
	a.RLock()
	defer a.RUnlock()
	return len(a.list)
}

type Corpus interface {
	FindClosestTatum(Tatum) AudioTatum
}