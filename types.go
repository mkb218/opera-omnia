package omnia

import "encoding/json"
import "sync"

// this is a raw audio file that has been uploaded. use libsox to determine its type and re-encode
// name comes from the form header
type AudioFile struct {
	data []byte
	name string
}

// an audio file not (by itself) bound for the corpus. This can be used to create audio from the corpus
type PostAudioFile struct {
	trackId string
	tatums []Tatum
}

// an audio file bound for (or pulled out of) the corpus
type CorpusAudioFile struct {
	trackId string
	fetchUrl string
	needData bool
	tatums []AudioTatum
}

// a tatum. This does not, by itself, belong in the corpus.
type Tatum struct {
	chroma, timbre []float64
	loudness_max float64
}

// an audiotatum. this belongs in the corpus.
// data is mp3-encoded, 64kbps, mono
type AudioTatum struct {
	Tatum
	data []byte
	trackId string
}

// type erasure :(
type queueNode struct {
	data json.Marshaler
	next *queueNode
}

// A Queue is like a buffered channel, with no maximum size
type Queue struct {
	head, tail *queueNode
	push, pop chan json.Marshaler
	sync.Mutex
	length chan int
}

func NewQueue() *Queue {
	a := new(Queue)
	a.push = make(chan json.Marshaler)
	a.pop = make(chan json.Marshaler)
	a.length = make(chan int)
	go func() {
		OUTER: for {
			select {
			case a.length <- 0:
			default:
			}
			a.Lock()
			f, ok := <- a.push
			if !ok {
				break
			}
			a.head = new(queueNode)
			a.tail = a.head
			a.head.data = f
			a.Unlock()
			closed := false
			length := 1
			for a.head != nil && !closed {
				a.Lock()
				t := a.head.data
				a.head = a.head.next
				select {
				case a.length <- length:
				default:
				}
				select {
				case a.pop <- t:
					length--
				case i, ok := <- a.push:
					if ok {
						a.tail.next = new(queueNode)
						a.tail = a.tail.next
						a.tail.data = i
						length++
					} else {
						a.pop <- t
						break OUTER
					}
				}
				a.Unlock()
			}
		}
		for a.head != nil {
			a.Lock()
			t := a.head.data
			a.head = a.head.next
			a.pop <- t
			a.Unlock()
		}
		close(a.pop)
	}()
	return a
}

func (a *Queue) Pop() (i json.Marshaler, ok bool) {
	i, ok = <- a.pop
	return
}

func (a *Queue) Push(i json.Marshaler) {
	a.push <- i
}

func (a *Queue) MarshalJSON() ([]byte, error) {
	a.Lock()
	out := []byte{'['}
	comma := ''
	for h := a.head; h != nil; {
		b, err := h.data.MarshalJSON()
		if err != nil {
			return nil, err
		}
		out = append(out, b..., comma)
		comma = ','
	}
	out = append(out, ']')
	return b, nil
}

func (a *Queue) Stop() {
	close(a.push)
}

func (a *Queue) Len() int {
	return <- a.length
}

type Corpus interface {
	FindClosestTatum(Tatum) AudioTatum
}