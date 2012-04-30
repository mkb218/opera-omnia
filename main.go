package main

import "runtime"
import "flag"
import "log"
import "log/syslog"
import "os"
import "net/http"

var address string = ":9001"
var logger string
var templateRoot = "/Users/mkb/code/opera-omnia/templates";

var gofuncs []func()

func init() {
	flag.StringVar(&templateRoot, "templates", templateRoot, "")
	flag.StringVar(&address, "address", address, "")
	flag.StringVar(&logger, "logfile", "", "")
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU()+1)
	InitIDForChecksum()
	InitSegmentsForChecksum()
	initAllSegs()
	log.SetFlags(log.LstdFlags|log.Lshortfile)
	if len(logger) > 0 {
		if logger != "-" {
			l, err := os.Create(logger)
			if err != nil {
				log.Println("Couldn't open logfile:", logger, err, ". using stderr")
			} else {
				defer l.Close()
				log.SetOutput(l)
			}
		} else {
			w, err := syslog.New(syslog.LOG_INFO, "")
			if err != nil {
				log.Println("couldn't open syslog daemon", err, "using stderr")
			} else {
				log.SetOutput(w)
			}
		}
	}
	err := os.MkdirAll(MapGobPath, 0700)
	if err != nil {
		log.Fatalln("couldn't create gob path",MapGobPath,err)
	}
	err = os.MkdirAll(dumppath, 0700)
	if err != nil {
		log.Fatalln("couldn't create",dumppath)
	}
	err = os.MkdirAll(samplepath, 0700)
	if err != nil {
		log.Fatalln("couldn't create",dumppath)
	}
	log.Println("I LIVE TO SERVE")
	for _, g := range gofuncs {
		go g()
	}
	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}