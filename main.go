package main

import "flag"
import "log"
import "os"
import "net/http"

var address string = ":9001"
var logger *string
var templateRoot = "/Users/mkb/code/opera-omnia/templates";

var gofuncs []func()

func init() {
	flag.StringVar(&templateRoot, "templates", templateRoot, "")
	flag.StringVar(&address, "address", address, "")
	logger = flag.String("logfile", "-", "")
}

func main() {
	flag.Parse()
	if logger != nil && *logger != "-" {
		l, err := os.Create(*logger)
		if err != nil {
			log.Print("Couldn't open logfile:", *logger, err, ". using stderr")
		} else {
			defer l.Close()
			log.SetOutput(l)
		}
	}
	log.Print("I LIVE TO SERVE")
	for _, g := range gofuncs {
		go g()
	}
	err := http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}