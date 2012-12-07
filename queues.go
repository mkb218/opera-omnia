package main

var queues = make(map[string]*Queue)
var queueProcs = make(map[string][]chan bool)

func init() {
	queues["upload"] = NewQueue() // type AudioFile
	queues["request"] = NewQueue() // type PostAudioFile
	queues["add"] = NewQueue() // type CorpusAudioFile
	
}