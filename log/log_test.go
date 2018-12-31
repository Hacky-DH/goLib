package log

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	logger := NewLogger()
	logger.SetLogger("console", nil)
	logger.SetLevel(DebugLevel)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Fatal("fatal")
	time.Sleep(time.Second)
}

func TestLogFile(t *testing.T) {
	logger := NewLogger()
	name := "test.log"
	config := make(map[string]interface{})
	config["path"] = name
	config["maxRollingTime"] = time.Second
	config["maxRollingNum"] = 1
	config["checkInterval"] = time.Second
	err := logger.SetLogger("file", config)
	if err != nil {
		t.Fatal("set logger error:", err)
	}
	logger.SetLevel(DebugLevel)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Fatal("fatal")

	time.Sleep(time.Second)

	if _, ok := FileExists(name); ok {
		fd, _ := os.Open(name)
		defer fd.Close()
		line := 0
		scanner := bufio.NewScanner(fd)
		for scanner.Scan() {
			str := scanner.Text()
			if !strings.Contains(str, "[rolling log]") {
				line++
				if !strings.HasSuffix(str, "debug") &&
					!strings.HasSuffix(str, "info") &&
					!strings.HasSuffix(str, "warn") &&
					!strings.HasSuffix(str, "error") &&
					!strings.HasSuffix(str, "fatal") {
					t.Error("log file error content:", str)
				}
			}
		}
		if line != 5 {
			t.Error("line num of log file expect = 5 but is", line)
		}
	} else {
		t.Error(name, "log file is not exists")
	}
}

func TestLogRollingFile(t *testing.T) {
	logger := NewLogger()
	config := make(map[string]interface{})
	config["path"] = "../../log/test.log"
	config["maxRollingTime"] = 10 * time.Second
	config["maxRollingNum"] = 5
	config["checkInterval"] = time.Second
	logger.SetLogger("file", config)
	logger.SetLevel(DebugLevel)

	for i := 0; i < 20; i++ {
		logger.Debug("debug %d", i)
		logger.Info("info %d", i)
		logger.Warn("warn %d", i)
		logger.Error("error %d", i)
		logger.Fatal("fatal %d", i)

		time.Sleep(1 * time.Second)
	}
}
