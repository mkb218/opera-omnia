package main

// ho shit here goes the crazy part
// using libshout and an external lame process

import "C"

import "flag"
import "os/exec"
import "strconv"

var bitrate int
var samplerate int
var server string
var port int
var user string
var password string
var dumpraw bool
var dumppath string

// compressed
type File struct {
	artist, title string
	data []byte
}

func (f *File) Write(p []byte) (n int, err error) {
	data = append(data, p...)
	return len(p), nil
}

var FileQueue chan File

func init() {
	flag.IntVar(&bitrate, "bitrate", 64, "kbps")
	flag.IntVar(&samplerate, "samplerate", 44100, "")
	flag.StringVar(&server, "server", "localhost", "")
	flag.IntVar(&port, "port", 8000, "")
	flag.StringVar(&user, "user", "", "")
	flag.StringVar(&password, "password", "", "")
	flag.BoolVar(&dumpraw, "dumpraw", false, "")
	flag.StringVar(&dumppath, "dumppath", "", "")
	FileQueue = make(chan File)
	gofuncs = append(gofuncs, FileProc)
	gofuncs = append(gofuncs, StreamProc)
}	

func StreamProc() {
	log.Print("starting StreamProc")
	// set stuff
	// connect to server
	for f := range FileQueue {
	}
}

func FileProc() {
	log.Print("starting FileProc")
	for ar := range AudioQueue {
		f := File{ar.artist, ar.title}
		p, err := os.LookPath("lame")
		if err != nil {
			log.Panic("no lame found! CAN'T STREAM. DYING.")
		}
		
		c := os.Command(p, "-r", "--bitwidth", "16", "--big-endian", "-b", strconv.Itoa(bitrate), "--cbr", "--nohist", "--signed", "-s", "44.1", "-", "-")
		c.Stdin = ar
		c.Stdout = f
		err = cmd.Wait()
		if err != nil {
			log.Print("encoding failed for ",ar.artist, ar.name, " :( ", err)
		}
		FileQueue <- f
	}
}

