package log

import (
	"testing"
	"time"
)

func TestGlob(t *testing.T) {
	tm := time.Now()
	bt := tm.Add(-time.Second * 10)
	m, err := Glob(".", "*.gz", bt)
	if err != nil {
		t.Error(err)
	}
	t.Log(m)
}
