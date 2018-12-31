package dns

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
   id:                   16bits
   header flags          16bits
   Question count        16bits
   Answer count          16bits
   Authorization count   16bits
   Addition count        16bits
   ref:https://tools.ietf.org/html/rfc1035
   ref:http://blog.csdn.net/tianxuhong/article/details/74922454
*/
type dnsHeader struct {
	Id                                 uint16
	Bits                               uint16
	Qdcount, Ancount, Nscount, Arcount uint16
}

var (
	idLock sync.Mutex
	idRand *rand.Rand
)

/* id returns a 16 bits random number to be used as a
   message id. The random provided should be good enough.
   ref:https://github.com/miekg/dns/blob/master/msg.go
*/
func id() uint16 {
	idLock.Lock()

	if idRand == nil {
		// This (partially) works around
		// https://github.com/golang/go/issues/11833 by only
		// seeding idRand upon the first call to id.

		var seed int64
		var buf [8]byte

		if _, err := crand.Read(buf[:]); err == nil {
			seed = int64(binary.LittleEndian.Uint64(buf[:]))
		} else {
			seed = rand.Int63()
		}

		idRand = rand.New(rand.NewSource(seed))
	}

	// The call to idRand.Uint32 must be within the
	// mutex lock because *rand.Rand is not safe for
	// concurrent use.
	//
	// There is no added performance overhead to calling
	// idRand.Uint32 inside a mutex lock over just
	// calling rand.Uint32 as the global math/rand rng
	// is internally protected by a sync.Mutex.
	id := uint16(idRand.Uint32())

	idLock.Unlock()
	return id
}

/*
   ref: http://www.cnblogs.com/chase-wind/p/6814053.html
   QR: 1bit 0 question, 1 response
   OperationCode: 4bits 0 query, 1 iquery,2 status
   AuthoritativeAnswer: 1bit just at response
   Truncation: 1bit
   RecursionDesired: 1bit
   RecursionAvailable: 1bit
   Z:3bits 0 reserve
   ResponseCode: 4bits
   QR opCode AA TC RD RA Z rcode
*/
func (header *dnsHeader) SetFlag(QR uint16, OperationCode uint16, AuthoritativeAnswer uint16,
	Truncation uint16, RecursionDesired uint16, RecursionAvailable uint16, ResponseCode uint16) {
	header.Bits = QR<<15 | OperationCode<<11 | AuthoritativeAnswer<<10 | Truncation<<9 |
		RecursionDesired<<8 | RecursionAvailable<<7 | ResponseCode
}

func (header *dnsHeader) ResponseCode() uint16 {
	return header.Bits & 0x000F
}

/*
	QuestionType: 1 get ipv4, 2 ns ..., 12 ptr 28 get ipv6
	QuestionClass: 1 internet
*/
type dnsQuery struct {
	QuestionType  uint16
	QuestionClass uint16
}

type dnsRR struct {
	Name    uint16 //duplicate name just is 2 bytes
	Type    uint16
	Class   uint16
	TTL     uint32
	DataLen uint16
}

type DnsResponse interface {
	Ips() []string
	TTL() uint32
	Time() time.Duration
}

type response struct {
	ips  []string
	ttl  uint32
	time time.Duration
}

func (r *response) Ips() []string {
	return r.ips
}
func (r *response) TTL() uint32 {
	return r.ttl
}
func (r *response) Time() time.Duration {
	return r.time
}

func parseDomainName(domain string) []byte {
	var (
		buffer   bytes.Buffer
		segments []string = strings.Split(domain, ".")
	)
	for _, seg := range segments {
		binary.Write(&buffer, binary.BigEndian, byte(len(seg)))
		binary.Write(&buffer, binary.BigEndian, []byte(seg))
	}
	binary.Write(&buffer, binary.BigEndian, byte(0x00))

	return buffer.Bytes()
}

//lookup domain on dnsServer, return DnsResponse
func Dig(dnsServer, domain string, timeout int) (DnsResponse, net.Error) {
	requestHeader := dnsHeader{
		Id:      id(),
		Qdcount: 1,
		Ancount: 0,
		Nscount: 0,
		Arcount: 0,
	}
	requestHeader.SetFlag(0, 0, 0, 0, 1, 0, 0)

	requestQuery := dnsQuery{
		QuestionType:  1, //ipv4
		QuestionClass: 1,
	}

	var (
		conn   net.Conn
		err    error
		buffer bytes.Buffer
		resLen int
	)

	//Err,Name,Server,IsTimeout,IsTemporary
	dnsErr := net.DNSError{"", domain, dnsServer, false, false}

	if conn, err = net.Dial("udp", dnsServer+":53"); err != nil {
		dnsErr.Err = err.Error()
		return nil, &dnsErr
	}
	defer conn.Close()
	now := time.Now()
	conn.SetDeadline(now.Add(time.Duration(timeout) * time.Second))
	conn.SetWriteDeadline(now.Add(time.Duration(timeout) * time.Second))
	conn.SetReadDeadline(now.Add(time.Duration(timeout) * time.Second))

	//only one domain name
	domainBytes := parseDomainName(domain)
	binary.Write(&buffer, binary.BigEndian, requestHeader)
	binary.Write(&buffer, binary.BigEndian, domainBytes)
	binary.Write(&buffer, binary.BigEndian, requestQuery)

	start := time.Now()
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		dnsErr.Err = err.Error()
		return nil, &dnsErr
	}

	buffer.Reset()
	buf := make([]byte, 1024)
	for {
		var length int
		if length, err = conn.Read(buf); err != nil {
			dnsErr.Err = err.Error()
			if strings.Contains(dnsErr.Err, "timeout") {
				dnsErr.IsTimeout = true
			}
			return nil, &dnsErr
		}
		resLen += length
		buffer.Write(buf)
		if length < 1024 {
			break
		}
	}
	digTime := time.Now().Sub(start)

	responseHeader := dnsHeader{}
	binary.Read(&buffer, binary.BigEndian, &responseHeader)
	if requestHeader.Id != responseHeader.Id {
		dnsErr.Err = "DNS response id and request id is not eq"
		return nil, &dnsErr
	}

	if responseHeader.ResponseCode() != 0 {
		dnsErr.Err = "DNS response error, code: " + strconv.Itoa(int(responseHeader.ResponseCode()))
		return nil, &dnsErr
	}

	if responseHeader.Qdcount != 1 {
		dnsErr.Err = "DNS response question number != 1"
		return nil, &dnsErr
	}

	if responseHeader.Ancount < 1 {
		dnsErr.Err = "DNS response answer number < 1"
		return nil, &dnsErr
	}

	var i uint16
	responseQuery := dnsQuery{}
	name := make([]byte, len(domainBytes))
	for i = 0; i < responseHeader.Qdcount; i++ {
		binary.Read(&buffer, binary.BigEndian, &name)
		binary.Read(&buffer, binary.BigEndian, &responseQuery)
	}

	rr := dnsRR{}
	ip := make([]byte, 4)
	res := response{
		ips:  make([]string, 0),
		ttl:  1<<32 - 1,
		time: digTime,
	}
	for i = 0; i < responseHeader.Ancount; i++ {
		binary.Read(&buffer, binary.BigEndian, &rr)
		if rr.TTL < res.ttl {
			res.ttl = rr.TTL
		}
		if rr.Type != 1 {
			// not ipv4
			data := make([]byte, rr.DataLen)
			binary.Read(&buffer, binary.BigEndian, &data)
			continue
		}
		if rr.DataLen != 4 {
			//DNS response invalid ip
			continue
		}
		binary.Read(&buffer, binary.BigEndian, &ip)
		res.ips = append(res.ips, net.IPv4(ip[0], ip[1], ip[2], ip[3]).String())
	}

	return &res, nil
}
