package perf

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestFloodRandomizedNames(t *testing.T) {
	lab := NewLab(t, Options{StartServer: true})
	const n = 64
	var ok atomic.Int64
	var wg sync.WaitGroup
	errCh := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("rand-%d.tools.lab.example.", i)
			q := model.Query{Name: model.Name(name), Type: model.TypeA, Class: model.ClassIN, Client: LabClient, Transport: model.TransportUDP, RD: true}
			raw, err := EncodeQuery(uint16(i+1), q, nil)
			if err != nil {
				errCh <- err.Error()
				return
			}
			out, err := DialUDP(lab.UDPAddr(), raw, time.Second)
			if err != nil {
				errCh <- name + ": " + err.Error()
				return
			}
			if out == nil {
				errCh <- name + ": no answer"
				return
			}
			msg, err := dnswire.UnpackUpstream(out)
			if err != nil {
				errCh <- name + ": " + err.Error()
				return
			}
			if msg.RCode != model.RCodeNoError || len(msg.Answers) == 0 {
				errCh <- fmt.Sprintf("%s: rcode=%s answers=%d", name, msg.RCode, len(msg.Answers))
				return
			}
			ok.Add(1)
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Error(e)
	}
	if ok.Load() != n {
		t.Fatalf("answered %d/%d", ok.Load(), n)
	}
}

func TestFloodEDNSSizePressure(t *testing.T) {
	lab := NewLab(t, Options{StartServer: true})
	q := QueryExact()
	raw := PackQuery(t, 42, q, &dnswire.EDNS{UDPSize: 4096})
	out := MustExchangeUDP(t, lab.UDPAddr(), raw)
	msg := Unpack(t, out)
	if msg.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", msg.RCode)
	}

	huge := make([]byte, 5000)
	copy(huge, raw)
	if got := ExchangeUDP(t, lab.UDPAddr(), huge, 150*time.Millisecond); got != nil {
		t.Fatalf("oversize packet produced %d bytes", len(got))
	}
}

func TestFloodAdmissionInflight(t *testing.T) {
	const capN = 2
	const extra = 4
	st := LabState("")
	// Hold admitted queries longer than a dropped datagram's wait would need,
	// so extras cannot sneak in after a slot is released.
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = 400 * time.Millisecond
	held := NewLab(t, Options{State: st, StartServer: true, MaxInflight: capN, QueryTimeout: time.Second})

	start := make(chan struct{})
	var wg sync.WaitGroup
	var answers, drops atomic.Int64
	for i := 0; i < capN+extra; i++ {
		wg.Add(1)
		go func(id uint16) {
			defer wg.Done()
			raw, err := EncodeQuery(id, QueryDelay(), nil)
			if err != nil {
				return
			}
			<-start
			out, err := DialUDP(held.UDPAddr(), raw, 800*time.Millisecond)
			if err != nil || out == nil {
				drops.Add(1)
				return
			}
			answers.Add(1)
		}(uint16(i + 1))
	}
	close(start)
	wg.Wait()
	if answers.Load() != capN {
		t.Fatalf("admitted answers=%d want %d (drops=%d); inflight cap not enforced", answers.Load(), capN, drops.Load())
	}
	if drops.Load() != extra {
		t.Fatalf("silent drops=%d want %d", drops.Load(), extra)
	}
}

func TestFloodTCPConnectionPressure(t *testing.T) {
	lab := NewLab(t, Options{
		StartServer: true,
		MaxTCPConns: 2,
		MaxTCPPerIP: 2,
	})
	var conns []net.Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < 2; i++ {
		c, err := net.Dial("tcp", lab.TCPAddr().String())
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	// Accept loop must observe both holders before the surplus dial.
	time.Sleep(30 * time.Millisecond)

	c, err := net.DialTimeout("tcp", lab.TCPAddr().String(), 300*time.Millisecond)
	if err != nil {
		// Refuse is also a cap; the usual path is accept-then-close.
		return
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(400 * time.Millisecond))
	raw := PackQuery(t, 7, QueryExact(), nil)
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(raw)))
	if _, err := c.Write(hdr[:]); err == nil {
		_, _ = c.Write(raw)
	}
	n, readErr := c.Read(hdr[:])
	if readErr == nil && n > 0 {
		t.Fatalf("surplus TCP connection returned %d bytes; MaxTCPConns=2 must close without a DNS response", n)
	}
	if readErr == nil {
		t.Fatal("surplus TCP connection stayed open and returned a DNS header")
	}
}
