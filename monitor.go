package main

import "sync"
import "flag"
import "time"
import "runtime"
import "log"
import "os"
import "os/signal"

var stackbuf []byte = make([]byte, 102400)
var logfile *os.File
var monitorlog string
var rotatelock sync.Mutex
var lastrotation time.Time

func dumpStackTrace() {
	rotatelock.Lock()
	defer rotatelock.Unlock()
	size := runtime.Stack(stackbuf, true)
	logfile.Write(stackbuf[0:size])
	logfile.Write([]byte("-----------------------------\n"))
}

func rotate() *os.File {
	rotatelock.Lock()
	defer rotatelock.Unlock()
	if logfile != nil {
		logfile.Close()
	}
	
	_, err := os.Stat(monitorlog)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Panicln("couldn't stat monitor log", err)
		} else {
			goto OPEN
		}
	}
	
	err = os.Rename(monitorlog, monitorlog+".old")
	if err != nil {
		log.Fatalln("couldn't rotate monitorlog file", err)
	}
	
	OPEN: f, err := os.Create(monitorlog)
	if err != nil {
		log.Fatalln("couldn't create monitorlog file", monitorlog, err)
	}
	return f
}

func monitor() {
	logfile = rotate()
	for {
		time.Sleep(15*time.Minute)
		dumpStackTrace()
	}
}

func signalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for {
		_ = <- c
		dumpStackTrace()
		close(RequestQueue)
		close(AudioQueue)
		close(FileQueue)
		close(UploadChan)
		os.Exit(2)
	}
}

func init() {
	flag.StringVar(&monitorlog, "monitorlog", "log", "")
	gofuncs = append(gofuncs, monitor)
	gofuncs = append(gofuncs, signalHandler)
}