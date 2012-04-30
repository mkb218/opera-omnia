package main

import "bytes"
import "log"
import "flag"
import "os/exec"
import "strconv"
// import "os"
import "path"
import "strings"

var bitrate int
var samplerate int
var server string
var port int
var user string
var password string
var mount string
var dumppath string

// compressed
// type File struct {
// 	artist, title string
// 	data []byte
// }

// func (f *File) Write(p []byte) (n int, err error) {
// 	f.data = append(f.data, p...)
// 	return len(p), nil
// }

// var FileQueue chan File

func init() {
	flag.IntVar(&bitrate, "bitrate", 64, "kbps")
	flag.IntVar(&samplerate, "samplerate", 44100, "")
	flag.StringVar(&dumppath, "dumppath", "/Users/mkb/code/opera-omnia/dump", "MUST EXIST")
//	FileQueue = make(chan File, 10)
	gofuncs = append(gofuncs, FileProc)
//	gofuncs = append(gofuncs, StreamProc)
}	

// func StreamProc() {
// 	log.Println("starting StreamProc")
// 	log.Println("FileQueue loop starts")
// 	for f := range FileQueue {
// 		log.Println("FileQueue")
// 		listenlock.RLock()
// 		p := f.artist + "-" + f.title + strconv.Itoa(listen.Count) + ".mp3"
// 		listen.Count++
// 		listenlock.RUnlock()
// 		p = path.Join(dumppath, strings.Replace(p, "/", "_", -1))
// 		file, err := os.Create(p)
// 		if err != nil {
// 			log.Println("creating raw dump file failed :(", err)
// 			return
// 		}
// 		defer file.Close()
// 		n, err := file.Write(f.data)
// 		if err != nil {
// 			log.Println("writing file failed!", err)
// 		} 
// 		log.Println("wrote",n,"bytes to dump file")
// 		file.Close()
// 	}
// 	
// 	log.Println("StreamProc exiting")
// 	
// }

func FileProc() {
	log.Println("starting FileProc")
	for ar := range AudioQueue {
		log.Println("from AudioQueue", ar.artist, ar.title)
//		f := File{ar.artist, ar.title, make([]byte,0)}
		p, err := exec.LookPath("lame")
		if err != nil {
			log.Panic("no lame found! CAN'T STREAM. DYING.")
		}
		
		f := ar.artist + "-" + ar.title + strconv.Itoa(listen.Count) + ".mp3"
		f = path.Join(dumppath, strings.Replace(p, "/", "_", -1))
		listen.Count++
		listenlock.RUnlock()
		c := exec.Command(p, "--tt", ar.title + " mangled by Opera Omnia", "--ta", ar.artist, "-r", "--bitwidth", "16", "--big-endian", "-b", strconv.Itoa(bitrate), "--cbr", "--nohist", "--signed", "-s", "44.1", "-", f)
		listenlock.RLock()
		c.Stdin = &ar
		c.Stdout = new(bytes.Buffer)
		b := new(bytes.Buffer)
		c.Stderr = b
		err = c.Run()
		if err != nil {
			log.Println("encoding failed for",ar.artist, ar.title, ":(", err)
			log.Println(string(b.Bytes()))
			log.Println(string(b.Bytes()))
			continue
		}
		log.Println("dumpchan send")
		dumpchan <- path.Base(f)
		log.Println("dumpchan sent")
	}
}

