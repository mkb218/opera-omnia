package main

import "time"
import "runtime"
import "log"
import "os"
import "os/signal"

var stackbuf []byte = make([]byte, 102400)
var logfile *os.File

func dumpStackTrace() {
	size := runtime.Stack(stackbuf, true)
	logfile.Write(stackbuf[0:size])
	logfile.Write([]byte("-----------------------------"))
}

func monitor() {
	for {
		time.Sleep(15*time.Second)
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
	gofuncs = append(gofuncs, monitor)
	// gofuncs = append(gofuncs, signalHandler)
	var err error
	logfile, err = os.Create("log")
	if err != nil {
		log.Panicln(err)
	}
}