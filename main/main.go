package main

import "flag"
import "log"

var address int = ":9001"
var logger *string

func init() {
	flag.StringVar(&address, "address", address, "")
	logger = flag.String("logfile", "-", "")
}

func main() {
	flag.Parse()
	if logger != nil && *logger != "-" {
		l, err := os.Create(logger)
		if err != nil {
			log.Print("Couldn't open logfile:", *logger, err, ". using stderr")
		} else {
			defer l.Close()
			log.SetOutput(l)
		}
	}
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}