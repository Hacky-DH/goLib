package log

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//ref https://github.com/kenshinx/godns/blob/master/log.go
//with log buffer
//add rolling file, see https://github.com/natefinch/lumberjack
//add compress

const log_output_buffer = 1024
const date_format = "2006-01-02"

const (
	DebugLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

var lock sync.RWMutex

type logMesg struct {
	Level int
	Mesg  string
}

type loggerHandler interface {
	Setup(config map[string]interface{}) error
	Write(mesg *logMesg)
	Rotate()
}

//no need add newline after msg
type Logger interface {
	SetLogger(handlerType string, config map[string]interface{}) error
	SetLevel(level int)
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
}

type LoggerImp struct {
	level   int
	mesgs   chan *logMesg
	outputs map[string]loggerHandler
}

func NewLogger() Logger {
	logger := &LoggerImp{
		mesgs:   make(chan *logMesg, log_output_buffer),
		outputs: make(map[string]loggerHandler),
	}
	go logger.run()
	return logger
}

func (l *LoggerImp) SetLogger(handlerType string, config map[string]interface{}) error {
	var handler loggerHandler
	switch handlerType {
	case "console":
		handler = newconsoleHandler()
	case "file":
		handler = newfileHandler()
	default:
		return errors.New("Unknown log handler.")
	}

	if err := handler.Setup(config); err != nil {
		return err
	}
	l.outputs[handlerType] = handler
	return nil
}

func (l *LoggerImp) SetLevel(level int) {
	l.level = level
}

func (l *LoggerImp) run() {
	for {
		select {
		case mesg := <-l.mesgs:
			for _, handler := range l.outputs {
				handler.Write(mesg)
			}
		}
	}
}

func (l *LoggerImp) writeMesg(mesg string, level int) {
	if l.level > level {
		return
	}

	for _, handler := range l.outputs {
		handler.Rotate()
	}

	lm := &logMesg{
		Level: level,
		Mesg:  mesg,
	}

	lock.RLock()
	defer lock.RUnlock()
	l.mesgs <- lm
}

func (l *LoggerImp) Debug(format string, v ...interface{}) {
	mesg := fmt.Sprintf("[DEBUG] "+format, v...)
	l.writeMesg(mesg, DebugLevel)
}

func (l *LoggerImp) Info(format string, v ...interface{}) {
	mesg := fmt.Sprintf("[INFO] "+format, v...)
	l.writeMesg(mesg, InfoLevel)
}

func (l *LoggerImp) Warn(format string, v ...interface{}) {
	mesg := fmt.Sprintf("[WARN] "+format, v...)
	l.writeMesg(mesg, WarnLevel)
}

func (l *LoggerImp) Error(format string, v ...interface{}) {
	mesg := fmt.Sprintf("[ERROR] "+format, v...)
	l.writeMesg(mesg, ErrorLevel)
}

func (l *LoggerImp) Fatal(format string, v ...interface{}) {
	mesg := fmt.Sprintf("[FATAL] "+format, v...)
	l.writeMesg(mesg, FatalLevel)
}

type consoleHandler struct {
	level  int
	logger *log.Logger
}

func newconsoleHandler() loggerHandler {
	return new(consoleHandler)
}

func (h *consoleHandler) Setup(config map[string]interface{}) error {
	if _level, ok := config["level"]; ok {
		level := _level.(int)
		h.level = level
	}
	h.logger = log.New(os.Stdout, "", log.LstdFlags)
	return nil
}

func (h *consoleHandler) Write(lm *logMesg) {
	if h.level <= lm.Level {
		h.logger.Println(lm.Mesg)
	}
}

func (h *consoleHandler) Rotate() {
	//do nothing
}

type fileHandler struct {
	logger         *log.Logger
	fileDesc       *os.File
	level          int
	logTime        int64
	fileName       string        //absolute path
	isCompress     bool          //default true
	isRollingFile  bool          //default true
	maxRollingTime time.Duration //default 7days
	maxRollingNum  int           //default 7, mean one day one file
	maxFileSize    int64         //default 100MB
	checkInterval  time.Duration //check if del old log files, default 45m
	rotateInterval time.Duration //maxRollingTime / maxRollingNum
	preRotateTime  time.Time
}

func newfileHandler() loggerHandler {
	return new(fileHandler)
}

func (h *fileHandler) Setup(config map[string]interface{}) error {
	if level, ok := config["level"]; ok {
		h.level = level.(int)
	}
	if compress, ok := config["isCompress"]; ok {
		h.isCompress = compress.(bool)
	} else {
		h.isCompress = true
	}
	if rolling, ok := config["isRollingFile"]; ok {
		h.isRollingFile = rolling.(bool)
	} else {
		h.isRollingFile = true
	}
	if roll, ok := config["maxRollingTime"]; ok {
		h.maxRollingTime = roll.(time.Duration)
	} else {
		h.maxRollingTime = 7 * 24 * time.Hour
	}
	if roll, ok := config["maxRollingNum"]; ok {
		h.maxRollingNum = roll.(int)
	} else {
		h.maxRollingNum = 7
	}
	if size, ok := config["maxFileSize"]; ok {
		h.maxFileSize = size.(int64)
	} else {
		h.maxFileSize = 100 * MB //100MB
	}
	h.rotateInterval = h.maxRollingTime / time.Duration(h.maxRollingNum)
	if interval, ok := config["checkInterval"]; ok {
		h.checkInterval = interval.(time.Duration)
	} else {
		h.checkInterval = 45 * time.Minute
	}

	if file, ok := config["path"]; ok {
		h.fileName, _ = filepath.Abs(file.(string))
		info, exist := FileExists(h.fileName)
		if exist {
			h.preRotateTime = info.ModTime()
			if h.isRollingFile {
				if h.isRotate() {
					h.rotate()
				} else {
					//append
					if err := h.newLogFile(true); err != nil {
						return err
					}
				}
			} else {
				//append
				if err := h.newLogFile(true); err != nil {
					return err
				}
			}
		} else {
			h.preRotateTime = time.Now()
			if err := h.newLogFile(false); err != nil {
				return err
			}
		}
	} else {
		return errors.New("Logger must config log path")
	}

	go h.logRolling()

	return nil
}

func (h *fileHandler) Write(lm *logMesg) {
	if h.logger == nil {
		return
	}

	if h.level <= lm.Level {
		h.logger.Println(lm.Mesg)
	}
}

func (h *fileHandler) write(level int, format string, v ...interface{}) {
	if h.logger == nil {
		return
	}

	if h.level <= level {
		msg := fmt.Sprintf("[DEBUG] "+format, v...)
		h.logger.Println(msg)
	}
}

func (h *fileHandler) newLogFile(append bool) error {
	flag := os.O_CREATE | os.O_WRONLY
	if append {
		flag = flag | os.O_APPEND
	} else {
		flag = flag | os.O_TRUNC
	}
	output, err := os.OpenFile(h.fileName, flag, 0644)
	if err != nil {
		return err
	}
	h.fileDesc = output
	h.logger = log.New(output, "", log.LstdFlags) //Lshortfile
	return nil
}

func (h *fileHandler) Rotate() {
	if !h.isRollingFile {
		return
	}
	//rolling files, compress or rename
	if h.isRotate() {
		if err := h.rotate(); err != nil {
			h.write(ErrorLevel, "rotate log file err %s", err)
		}
	}
}

func (h *fileHandler) logRolling() {
	if !h.isRollingFile {
		return
	}
	h.delOldFiles()
	for range time.Tick(h.checkInterval) {
		h.delOldFiles()
	}
}

func (h *fileHandler) delOldFiles() error {
	//delete old log files
	dir, _ := filepath.Split(h.fileName)
	var pattern string
	if h.isCompress {
		pattern = "*.gz"
	} else {
		pattern = "*.log"
	}
	now := time.Now()
	bt := now.Add(-h.maxRollingTime)
	if dir == "" {
		dir = "."
	}
	delFiles, err := Glob(dir, pattern, bt)
	if err != nil {
		h.write(ErrorLevel, "[rolling log] find old log file in dir %s err %s", dir, err)
		return err
	}
	if len(delFiles) > 0 {
		h.write(DebugLevel, "[rolling log] find %d old log file(s) in dir %s", len(delFiles), dir)
		for _, file := range delFiles {
			if err := os.Remove(file); err == nil {
				h.write(DebugLevel, "delete log file %s done", filepath.Base(file))
			}
		}
	}
	return nil
}

func (h *fileHandler) isRotate() bool {
	//check file size
	info, exist := FileExists(h.fileName)
	if exist {
		if info.Size() > h.maxFileSize {
			return true
		}
	}
	//check file time
	now := time.Now()
	next := h.preRotateTime.Add(h.rotateInterval)
	gap := now.Sub(next)
	return gap > 0 || (gap < 0 && gap > -100*time.Millisecond)
}

//assume fileName is exists
func (h *fileHandler) rotate() error {
	lock.Lock()
	defer lock.Unlock()
	if h.fileDesc != nil {
		if err := h.fileDesc.Close(); err != nil {
			return err
		}
		h.fileDesc = nil
	}
	now := time.Now()
	suffix := "-" + now.Format("20060102-150405")
	if h.isCompress {
		if err := Compress(h.fileName, suffix, true); err != nil {
			return err
		}
	} else {
		if err := os.Rename(h.fileName, h.fileName+suffix+".log"); err != nil {
			return err
		}
	}
	if err := h.newLogFile(false); err != nil {
		return err
	}
	h.preRotateTime = time.Now()
	//h.write(DebugLevel, "[rolling log] rotate to a new log file")
	return nil
}
