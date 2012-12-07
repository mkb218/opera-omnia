package main

import "runtime"
import "flag"
import "os"
import "net/http"

var address string = ":9001"
var logger string
var templateRoot = "./templates";
var debug bool

var gofuncs []func()
var initFuncs []func()

func init() {
	flag.StringVar(&templateRoot, "templates", templateRoot, "")
	flag.StringVar(&address, "address", address, "")
	flag.BoolVar(&debug, "debug", false, "")
}

func main() {
	flag.Parse()
	for _, i := range initFuncs {
		i()
	}
	runtime.GOMAXPROCS(runtime.NumCPU()+1)
	for _, g := range gofuncs {
		go g()
	}
	logMessage(LOG_INFO, "I LIVE TO SERVE")
	err = http.ListenAndServe(address, nil)
	if err != nil {
		logMessage(LOG_ERROR, "ListenAndServe:", err)
	}
}