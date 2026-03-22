package caster

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type Client struct {
	host         string
	port         int
	pass         string
	user         string
	mount        string
	ntripVersion int
	dialTO       time.Duration
	writeTO      time.Duration
	mu           sync.Mutex
	conn         net.Conn
	connected    bool
}

func New(host string, port int, pass, mount string) *Client {
	return &Client{
		host:    host,
		port:    port,
		pass:    pass,
		mount:   mount,
		dialTO:  8 * time.Second,
		writeTO: 8 * time.Second,
	}
}

func NewWithAuth(host string, port int, user, pass, mount string, ntripVersion int) *Client {
	return &Client{
		host:         host,
		port:         port,
		user:         user,
		pass:         pass,
		mount:        mount,
		ntripVersion: ntripVersion,
		dialTO:       8 * time.Second,
		writeTO:      8 * time.Second,
	}
}

func (c *Client) Send(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnected(); err != nil {
		return err
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTO)); err != nil {
		c.closeLocked()
		return err
	}
	_, err := c.conn.Write(frame)
	if err == nil {
		return nil
	}

	c.closeLocked()
	if err := c.ensureConnected(); err != nil {
		return err
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTO)); err != nil {
		c.closeLocked()
		return err
	}
	_, err = c.conn.Write(frame)
	if err != nil {
		c.closeLocked()
	}
	return err
}

func (c *Client) ensureConnected() error {
	if c.connected && c.conn != nil {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := net.DialTimeout("tcp", addr, c.dialTO)
	if err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(c.writeTO)); err != nil {
		_ = conn.Close()
		return err
	}

	mount := c.mount
	if len(mount) > 0 && mount[0] != '/' {
		mount = "/" + mount
	}

	var head string
	if c.ntripVersion == 2 && c.user != "" {
		// NTRIP v2 with basic auth
		auth := base64.StdEncoding.EncodeToString([]byte(c.user + ":" + c.pass))
		head = fmt.Sprintf("GET /%s HTTP/1.1\r\n", mount[1:])
		head += fmt.Sprintf("Authorization: Basic %s\r\n", auth)
		head += "Ntrip-Version: Ntrip/2.0\r\n"
		head += "User-Agent: go-gateway/1.0\r\n"
		head += "\r\n"
	} else {
		// NTRIP v1
		head = fmt.Sprintf("SOURCE %s %s\r\nSource-Agent: go-gateway\r\n\r\n", c.pass, mount)
		log.Printf("[NTRIP] connecting to %s:%d mount=%s ntrip_version=%d", c.host, c.port, mount, c.ntripVersion)
	}

	if _, err := conn.Write([]byte(head)); err != nil {
		log.Printf("[NTRIP] write failed: %v", err)
		_ = conn.Close()
		return err
	}

	buf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("[NTRIP] response read error: %v", err)
		_ = conn.Close()
		return err
	}
	log.Printf("[NTRIP] caster response: %s", string(buf[:n]))

	c.conn = conn
	c.connected = true
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
	return nil
}

func (c *Client) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.connected = false
}
