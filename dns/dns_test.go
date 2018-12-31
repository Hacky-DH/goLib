package dns

import (
	"testing"
)

func TestDns(t *testing.T) {
	res, err := Dig("114.114.114.114", "www.example.com", 3)
	if err != nil {
		t.Error(err.Error())
	}
	if res != nil {
		t.Log(res.Ips(), res.TTL(), res.Time())
	}
}

func BenchmarkDns(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Dig("114.114.114.114", "www.example.com", 3)
	}
}
