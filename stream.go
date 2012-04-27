package main

// #cgo LDFLAGS: -lshout
// #include <shout/shout.h>
// #include <stdlib.h>
import "C"

import "bytes"
import "log"
import "flag"
import "os/exec"
import "strconv"
import "runtime"
import "unsafe"
import "time"
import "os"
import "path"
import "strings"

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
	flag.BoolVar(&dumpraw, "dumpraw", true, "")
	flag.StringVar(&dumppath, "dumppath", "/Users/mkb/code/opera-omnia/dump", "MUST EXIST")
	FileQueue = make(chan File)
	gofuncs = append(gofuncs, FileProc)
	gofuncs = append(gofuncs, StreamProc)
}	

func StreamProc() {
	log.Println("starting StreamProc")
	runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1)+1)
	runtime.LockOSThread()
	C.shout_init()
	shout_ok := false
	shout := C.shout_new()
	i := 0
	{
		if shout == nil {
			log.Println("couldn't allocate shout_t")
			goto LOOP
		}
		chost := C.CString(server)
		defer C.free(unsafe.Pointer(chost))
		if C.shout_set_host(shout, chost) != C.SHOUTERR_SUCCESS {
			g := C.GoString(C.shout_get_error(shout))
			log.Println("couldn't set host", g)
			goto LOOP
		}
		if C.shout_set_protocol(shout, C.SHOUT_PROTOCOL_ICY) != C.SHOUTERR_SUCCESS {
			g := C.GoString(C.shout_get_error(shout))
			log.Printf("Error setting protocol: %s\n", g);
			goto LOOP
		}

		if C.shout_set_port(shout, C.ushort(port)) != C.SHOUTERR_SUCCESS {
			log.Printf("Error setting port: %s\n", C.GoString(C.shout_get_error(shout)));
			goto LOOP
		}

		cuser := C.CString(user)
		defer C.free(unsafe.Pointer(cuser))
		if C.shout_set_user(shout, cuser) != C.SHOUTERR_SUCCESS {
			log.Printf("Error setting user: %s\n", C.GoString(C.shout_get_error(shout)));
			goto LOOP
		}

		cpassword := C.CString(password)
		defer C.free(unsafe.Pointer(cpassword))
		if C.shout_set_password(shout, cpassword) != C.SHOUTERR_SUCCESS {
			log.Printf("Error setting password: %s\n", C.GoString(C.shout_get_error(shout)));
			goto LOOP
		}

		if C.shout_set_format(shout, C.SHOUT_FORMAT_MP3) != C.SHOUTERR_SUCCESS {
			log.Printf("Error setting user: %s\n", C.GoString(C.shout_get_error(shout)));
			goto LOOP
		}

		// connect to server
		if C.shout_open(shout) != C.SHOUTERR_SUCCESS {
			log.Printf("Couldn't open streaming server: %s", C.GoString(C.shout_get_error(shout)))
			goto LOOP
		}
		shout_ok = true
	}
		
	LOOP: for f := range FileQueue {
		if dumpraw {
			go func() {
				p := f.artist + "-" + f.title + strconv.Itoa(i) + ".mp3"
				i++
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
				log.Println("wrote",n,"bytes")
			}()
		}
		if shout_ok {
			r := C.shout_send(shout, (*C.uchar)(unsafe.Pointer(&f.data[0])), C.size_t(len(f.data)))
			if r != C.SHOUTERR_SUCCESS {
				log.Println("send error", C.GoString(C.shout_get_error(shout)))
			}
			sleeptime := C.shout_delay(shout)
			time.Sleep(time.Duration(sleeptime) * time.Millisecond)
		}
		i++
	}
	
	C.shout_close(shout) // what if there's an error? WHO CARES I'M DYING
	C.shout_shutdown()
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
		
		c := exec.Command(p, "-r", "--bitwidth", "16", "--big-endian", "-b", strconv.Itoa(bitrate), "--cbr", "--nohist", "--signed", "-s", "44.1", "-", "-")
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
		FileQueue <- f
	}
}

