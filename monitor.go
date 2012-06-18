package main

import "math"
import "fmt"
import "sync"
import "flag"
import "time"
import "runtime"
import "log"
import "os"
import "io"
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

func dumpHisto(w io.Writer) {
	ss := make([][]int, 25)
	for i := range ss {
		ss[i] = make([]int, 40)
	}
	func () {
		allSegsLock.RLock()
		defer allSegsLock.RUnlock()
		
		for _, v := range allSegs.Segs {
			for i := 0; i < 12; i++ {
				trg := int(v.Pitches[i] * 40)
				if trg == 40 {
					trg--
				}
				ss[i][trg]++
			}
			for i := 0; i < 12; i++ {
				t := int(math.Log10(math.Abs(v.Timbre[i])+1))
				if v.Timbre[i] > 0 {
					t += 20
				} else {
					t = 19 - t
				}
				if t < 0 {
					t = 0
				}
				if t > 39 {
					t = 39
				}
				
				ss[i+12][t]++
			}
			trg := int(v.BeatDistance * 40)
			if trg >= 40 {
				trg = 39
			}			
			ss[24][trg]++
		}
	}()
	for _, g := range ss {
		max := 0
		for _, v := range g {
			if v > max {
				max = v
			}
		}
		for h := max; h > 0; h-- {
			for v := range g {
				if v >= h {
					w.Write([]byte{'#'})
				} else {
					w.Write([]byte{' '})
				}
			}
			w.Write([]byte{'\n'})
		}
		w.Write([]byte("====\n"))
	}
}

func monitor() {
	logfile = rotate()
	for {
		fmt.Fprintln(logfile, time.Now())
		fmt.Fprintf(logfile, "dumpchan %d\n", len(dumpchan))
		fmt.Fprintf(logfile, "RequestQueue %d\n", len(RequestQueue))
		fmt.Fprintf(logfile, "AudioQueue %d\n", len(AudioQueue))
		fmt.Fprintf(logfile, "UploadChan %d\n", len(UploadChan))
		// dumpHisto(logfile)
		fmt.Fprintln(logfile,"----")
		time.Sleep(5*time.Minute)
	}
}

func signalHandler() {
	c := make(chan os.Signal, 100)
	signal.Notify(c, os.Interrupt)
	for {
		_ = <- c
		dumpStackTrace()
		close(RequestQueue)
		close(AudioQueue)
		// close(FileQueue)
		close(UploadChan)
		os.Exit(2)
	}
}

func init() {
	flag.StringVar(&monitorlog, "monitorlog", "log", "")
	gofuncs = append(gofuncs, monitor)
	gofuncs = append(gofuncs, signalHandler)
}