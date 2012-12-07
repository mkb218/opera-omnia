package main

import "log"
import "fmt"
import "flag"
import "sync"
import "io"
import "os"
import "time"

var logfile *os.File

var logfilename string
var rotationperiod uint
var logstokeep uint
var maxlogsize int64
var loglevel uint

const (LOG_FATAL = 0
	LOG_ERROR = 1
	LOG_WARN = 2
	LOG_INFO = 3
	LOG_DEBUG = 4)

func init() {
	flag.StringVar(&logfilename, "logfile", "/var/log/opera-omnia.log", "basename of log file")
	flag.UintVar(&rotationperiod, "rotationperiod", 24, "rotation period of log file (hours). logs always rotated at startup")
	flag.UintVar(&logstokeep, "logstokeep", 7, "log files to keep before deleting")
	flag.Int64Var(&maxlogsize, "maxlogsize", 10000000, "maximum size of log file before rotation")
	flag.UintVar(&loglevel, "loglevel", 3, "only log messages with lower or equal priority will be printed (0-4)")
	initFuncs = append(initFuncs, initLogrotation)
}

func initLogrotation() {
	log.SetFlags(log.LstdFlags|log.Lshortfile)
	logrotate()
}

func logMessage(level uint, msg interface{}...) {
	if level == LOG_FATAL {
		logger.Panic(msg...)
	} else if level <= loglevel {
		logger.Print("Level:", level, msg...)
	}
	logrotate()
}

func logrotate() {
	if logfile != nil {
		if fi := logfile.Stat(); fi.Size() < maxlogsize || time.Since(fi.ModTime()) < time.Duration(rotationperiod * time.Hour) {
			return
		}
	}
	lastfile := fmt.Sprintf("%s.%u", logfilename, logstokeep)
	if _, err := os.Stat(lastfile); err != nil {
		err = os.Remove(fmt.Sprintf("%s.%u", logfilename, logstokeep))
		if err != nil {
			logger.Print("ERROR: couldn't remove last log file:", err)
		}
	}

	for i := logstokeep; i > 0; i-- {
		iminusname := fmt.Sprintf("%s.%u", logfilename, i-1)
		if fi, err := os.Stat(); err != nil {
			err = os.Rename(iminusname, lastfile)
			if err != nil {
				logger.Print("ERROR: couldn't rotate file", i, ":", err)
			}
		}
		lastfile = iminusname
	}
	
	tmpname := logfilename+".tmp"
	t, err := os.Create(tmpname)
	if err != nil {
		logger.Print("ERROR: couldn't create temporary log file", err)
	}
	
	log.SetOutput(t)

	if logfile != nil {
		err = logfile.Close()
		if err != nil {
			logger.Print("ERROR: couldn't close old logfile!", err)
		}
	}
	logfile = t
	err = os.Rename(logfilename, logfilename+".0")
	if err != nil {
		logger.Print("ERROR: couldn't rename old logfile!", err)
	}
	err = os.Link(tmpname, logfilename)
	if err != nil {
		logger.Print("ERROR: couldn't link tmpfile to real name", err)
	}
	err = os.Remove(tmpname)
	if err != nil {
		logger.Print("ERROR: couldn't remove tmp link!", err)
	}
}



