package main

import "omnia"

var queues = make(map[string]*Queue)
var queueProcs = make(map[string][]chan bool)

func StartQueues() {
	omnia.ParseConf()
	bufsize := omnia.Conf["bufsize"]
	queues["upload"] = omnia.NewQueue(bufsize) // type AudioFile
	queues["request"] = omnia.NewQueue(bufsize) // type PostAudioFile
	queues["add"] = omnia.NewQueue(bufsize) // type CorpusAudioFile	
}

type q struct{}

func AudioFileProcessor(quitchan chan q) {
	go func() {
		for {
			select {
			case <- quitchan:
				return
			}
			q, ok := queues["upload"].Pop()
			if !ok {
				return
			}
			af := q.(AudioFile) // panic is good here
			// submit to echonest
			trackid, tatums := AnalyzeFile(af)
			if af.Corpus {
				at := FillAudioTatums(af, trackid, tatums)
				queues["add"].Push(at)
			}
			if af.Request {
				paf := omnia.PostAudioFile{TrackId:trackid, InfoUrl:af.InfoUrl, Tatums:tatums}
				queues["request"].Push(paf)
			}
		}
	}()
	return c
}