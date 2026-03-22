package capture

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

type FrameHandler func(sourceKey, sourceIP string, frame []byte, at time.Time)

type TCPConfig struct {
	ListenPort int
	QueueSize  int
}

type TCPListener struct {
	cfg     TCPConfig
	handler FrameHandler
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

func NewTCPListener(cfg TCPConfig, handler FrameHandler) *TCPListener {
	return &TCPListener{cfg: cfg, handler: handler}
}

func (l *TCPListener) Run(ctx context.Context) {
	addr := fmt.Sprintf(":%d", l.cfg.ListenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[TCP] listen %s: %v", addr, err)
	}
	log.Printf("[TCP] listening on %s (TCP mode, no libpcap)", addr)

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		<-ctx.Done()
		ln.Close()
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("[TCP] accept error: %v", err)
				continue
			}
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			l.handleConn(ctx, c)
		}(conn)
	}
}

func (l *TCPListener) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	srcAddr := conn.RemoteAddr().String()
	log.Printf("[TCP] client connected: %s", srcAddr)
	defer log.Printf("[TCP] client closed: %s", srcAddr)

	scanner := rtcm.NewScanner()
	buf := make([]byte, 32*1024)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return
		}

		n, err := conn.Read(buf)
		if n > 0 {
			frames := scanner.Push(buf[:n])
			now := time.Now()
			for _, fr := range frames {
				if l.handler != nil {
					l.handler(srcAddr, srcAddr, fr, now)
				}
			}
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if err != io.EOF {
				log.Printf("[TCP] read error %s: %v", srcAddr, err)
			}
			return
		}
	}
}

func (l *TCPListener) Wait() {
	l.wg.Wait()
}
