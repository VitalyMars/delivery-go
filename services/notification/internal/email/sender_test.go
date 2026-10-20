package email

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSMTPRespectsCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buffer := make([]byte, 1)
		_, _ = conn.Read(buffer)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := NewSender(host, port, "sender@example.com").Send(ctx, "user@example.com", "Subject", "Body"); err == nil {
		t.Fatal("accepted silent SMTP server")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("SMTP ignored cancellation")
	}
	listener.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("connection was not closed")
	}
}
