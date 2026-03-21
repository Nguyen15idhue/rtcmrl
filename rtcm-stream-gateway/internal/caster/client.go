package caster

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Client struct {
	host      string
	port      int
	pass      string
	mount     string
	dialTO    time.Duration
	writeTO   time.Duration
	mu        sync.Mutex
	conn      net.Conn
	connected bool
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
	head := fmt.Sprintf("SOURCE %s %s\r\nSource-Agent: go-gateway\r\n\r\n", c.pass, mount)
	if _, err := conn.Write([]byte(head)); err != nil {
		_ = conn.Close()
		return err
	}

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
