package util

import (
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"
)

const (
	_ = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

var (
	ErrProcessExist = errors.New("same process is exists")
	ErrExit = errors.New("process will exit")
)

//file exists
func FileExists(name string) (os.FileInfo, bool) {
	info, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return info, false
		}
	}
	return info, true
}

//glob searches for files in dir, matching pattern and before time
//exclude dirs in dir
func Glob(dir, pattern string, beforeTime time.Time) ([]string, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, err
	}
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, err
	}

	m := make([]string, 0)
	for _, info := range fis {
		if info.IsDir() {
			continue
		}
		matched, err := filepath.Match(pattern, info.Name())
		if err != nil {
			return m, err
		}
		if matched && info.ModTime().Before(beforeTime) {
			m = append(m, filepath.Join(dir, info.Name()))
		}
	}
	return m, nil
}

func Compress(filename string, suffix string, del bool) error {
	in, err := os.Open(filename)
	if err != nil {
		return err
	}
	out, err := os.Create(filename + suffix + ".gz")
	if err != nil {
		return err
	}
	defer out.Close()
	gzout := gzip.NewWriter(out)
	defer gzout.Close()
	_, err = io.Copy(gzout, in)
	if err != nil {
		return err
	}
	in.Close()
	if del {
		err = os.Remove(filename)
	}
	return err
}

//go tool pprof cpu.cprof
func StartProfileCPU(name string) (*os.File, error) {
	f, err := os.Create(name + ".cprof")
	if err != nil {
		return f, err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return f, nil
	}
	return f, nil
}

func StopProfileCPU(fd *os.File) {
	pprof.StopCPUProfile()
	if fd != nil {
		fd.Close()
	}
}

func ProfileMEM(name string) error {
	f, err := os.Create(name + ".mprof")
	if err != nil {
		return err
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return err
	}
	return nil
}

func ProcessExist(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if runtime.GOOS == "windows" {
		return true
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	return true
}

//CreatePidFile return err when other process is exist or other error
//ref https://github.com/tabalt/pidfile
func CreatePidFile(path string) error {
	pid, err := ReadPidFromFile(path)
	if err == nil && ProcessExist(pid) {
		return ErrProcessExist
	}
	thisPid := os.Getpid()
	fb := []byte(strconv.Itoa(thisPid))
	if err := ioutil.WriteFile(path, fb, 0666); err != nil {
		return err
	}
	return nil
}

func ReadPidFromFile(path string) (int, error) {
	fb, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	id, err := strconv.Atoi(string(fb))
	if err != nil || id <= 0 {
		return 0, errors.New("pid file data error")
	}

	return id, nil
}
