package main

import "bytes"
import "log"
import "flag"
import "os/exec"
import "strconv"
import "runtime"
import "syscall"
import "os"
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
type File struct {
	artist, title string
	data []byte
}

func (f *File) Write(p []byte) (n int, err error) {
	f.data = append(f.data, p...)
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
	flag.StringVar(&dumppath, "dumppath", "/Users/mkb/code/opera-omnia/dump", "MUST EXIST")
	flag.StringVar(&mount, "mount", "/opera-omnia", "")
	FileQueue = make(chan File, 10)
	gofuncs = append(gofuncs, FileProc)
	gofuncs = append(gofuncs, StreamProc)
}	

func PlaylistProc(c chan string) {
	fifo, err := os.OpenFile("ices.pipe", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		log.Fatalln("couldn't open fifo!", err)
	}
	for {
		p := <-c
		fifo.Write([]byte(p))
		fifo.Write([]byte{'\n'})
	}
}


func StreamProc() {
	log.Println("starting StreamProc")
	runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1)+1)
	runtime.LockOSThread()
	// mkfifo
	playlistc := make(chan string)
	go PlaylistProc(playlistc)
	icesp, err := exec.LookPath("ices0")
	if err != nil {
		log.Fatalln("couldn't find ices!", err)
	}
	iceargs := []string{"-h", server, "-p", strconv.Itoa(port), "-P", password, "-m", mount, "-S", "perl", "-n", "Opera Omnia", "-u", "http://opera-omnia.hydrogenproject.com"}
	ices := exec.Command(icesp, iceargs...)
	ices.Stdout = os.Stdout
	ices.Stderr = os.Stderr
	i := 0
	log.Println("FileQueue loop starts")
	for f := range FileQueue {
		log.Println("FileQueue")
		p := f.artist + "-" + f.title + strconv.Itoa(i) + ".mp3"
		p = path.Join(dumppath, strings.Replace(p, "/", "_", -1))
		file, err := os.Create(p)
		if err != nil {
			log.Println("creating raw dump file failed :(", err)
			return
		}
		defer file.Close()
		n, err := file.Write(f.data)
		if err != nil {
			log.Println("writing file failed!", err)
		} 
		log.Println("wrote",n,"bytes to dump file")
		file.Close()
		if ices.ProcessState == nil || ices.ProcessState.Exited() {
			if ices.ProcessState != nil {
				ices.Wait()
				ws := ices.ProcessState.Sys().(syscall.WaitStatus)
				log.Println("ices exited with status", ws.ExitStatus())
				// log.Println("stdout", string(ices.Stdout.(*bytes.Buffer).Bytes()))
				// log.Println("stderr", string(ices.Stdout.(*bytes.Buffer).Bytes()))
				// ices.Stdout = new(bytes.Buffer)
				// ices.Stderr = new(bytes.Buffer)
				ices = exec.Command(icesp, iceargs...)
			}
			err := ices.Start()
			if err != nil {
				log.Fatalln("couldn't start ices", err)
			}
			log.Println("started ices")
		}
		playlistc <- p
		i++
	}
	
	log.Println("StreamProc exiting")
	
}

func FileProc() {
	log.Println("starting FileProc")
	for ar := range AudioQueue {
		log.Println("from AudioQueue", ar.artist, ar.title)
		f := File{ar.artist, ar.title, make([]byte,0)}
		p, err := exec.LookPath("lame")
		if err != nil {
			log.Panic("no lame found! CAN'T STREAM. DYING.")
		}
		
		c := exec.Command(p, "--tt", ar.title + " mangled by Opera Omnia", "--ta", ar.artist, "-r", "--bitwidth", "16", "--big-endian", "-b", strconv.Itoa(bitrate), "--cbr", "--nohist", "--signed", "-s", "44.1", "-", "-")
		c.Stdin = &ar
		c.Stdout = &f
		b := new(bytes.Buffer)
		c.Stderr = b
		err = c.Run()
		if err != nil {
			log.Println("encoding failed for",ar.artist, ar.title, ":(", err)
			log.Println(string(b.Bytes()))
			continue
		}
		log.Println("sending to filequeue")
		FileQueue <- f
		log.Println("sent to filequeue")
	}
}

