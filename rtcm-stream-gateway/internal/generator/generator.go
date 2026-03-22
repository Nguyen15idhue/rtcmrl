package generator

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

type Config struct {
	Host       string
	Port       int
	StationIDs []int
	IntervalMs int
	FrameType  uint16
}

type Generator struct {
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
	conns   map[int]net.Conn
}

func New(cfg Config) *Generator {
	if cfg.IntervalMs <= 0 {
		cfg.IntervalMs = 1000
	}
	if cfg.FrameType == 0 {
		cfg.FrameType = 1006
	}
	if len(cfg.StationIDs) == 0 {
		cfg.StationIDs = []int{1, 2, 3, 1001, 2001}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Generator{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		conns:  make(map[int]net.Conn),
	}
}

func (g *Generator) Start() error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = true
	g.mu.Unlock()

	addr := g.cfg.Host
	if addr == "" {
		addr = "127.0.0.1"
	}
	target := net.JoinHostPort(addr, "12101")

	conn, err := net.Dial("tcp", target)
	if err != nil {
		g.mu.Lock()
		g.running = false
		g.mu.Unlock()
		return err
	}

	for _, sid := range g.cfg.StationIDs {
		g.conns[sid] = conn
	}

	go g.run()
	log.Printf("[GEN] started: target=%s stations=%v interval=%dms", target, g.cfg.StationIDs, g.cfg.IntervalMs)
	return nil
}

func (g *Generator) Stop() {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return
	}
	g.running = false
	g.mu.Unlock()

	g.cancel()

	for _, conn := range g.conns {
		if conn != nil {
			conn.Close()
		}
	}
	g.mu.Lock()
	g.conns = make(map[int]net.Conn)
	g.mu.Unlock()

	log.Printf("[GEN] stopped")
}

func (g *Generator) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

func (g *Generator) GetConfig() Config {
	return g.cfg
}

func (g *Generator) SetConfig(cfg Config) {
	g.mu.Lock()
	g.cfg = cfg
	g.mu.Unlock()
}

func (g *Generator) run() {
	ticker := time.NewTicker(time.Duration(g.cfg.IntervalMs) * time.Millisecond)
	defer ticker.Stop()

	frameNum := 0
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			for _, sid := range g.cfg.StationIDs {
				frame := generateFrame(sid, frameNum, g.cfg.FrameType)
				g.mu.RLock()
				conn := g.conns[sid]
				g.mu.RUnlock()

				if conn != nil {
					conn.Write(frame)
				}
			}
			frameNum++
		}
	}
}

func generateFrame(stationID, frameNum int, msgType uint16) []byte {
	body := buildMsgBody(stationID, frameNum, msgType)
	return rtcm.BuildFrame(msgType, uint16(stationID), body)
}

func buildMsgBody(stationID, frameNum int, msgType uint16) []byte {
	switch msgType {
	case 1005:
		return buildMsg1005(stationID, frameNum)
	case 1006:
		return buildMsg1006(stationID, frameNum)
	default:
		return buildMsg1006(stationID, frameNum)
	}
}

func buildMsg1005(stationID, frameNum int) []byte {
	body := make([]byte, 14)

	x := -2697745.0 + float64(stationID)*0.1
	y := 5019520.0 + float64(stationID)*0.1
	z := 2564650.0 + float64(stationID)*0.1

	writeScaledFloat(body, 0, x, 0.0001)
	writeScaledFloat(body, 38, y, 0.0001)
	writeScaledFloat(body, 76, z, 0.0001)

	return body
}

func buildMsg1006(stationID, frameNum int) []byte {
	body := make([]byte, 17)

	body[0] = 0x00

	x := -2697745.0 + float64(stationID)*0.1
	y := 5019520.0 + float64(stationID)*0.1
	z := 2564650.0 + float64(stationID)*0.1
	h := 100.0

	writeScaledFloat(body, 8, x, 0.0001)
	writeScaledFloat(body, 46, y, 0.0001)
	writeScaledFloat(body, 84, z, 0.0001)
	writeScaledFloat(body, 122, h, 0.0001)

	return body
}

func writeScaledFloat(b []byte, bitPos int, val float64, scale float64) {
	scaled := int64(val / scale)

	for i := 0; i < 38; i++ {
		bit := (scaled >> (37 - i)) & 1
		byteIdx := (bitPos + i) / 8
		bitInByte := 7 - ((bitPos + i) % 8)
		if byteIdx < len(b) {
			if bit == 1 {
				b[byteIdx] |= 1 << bitInByte
			} else {
				b[byteIdx] &^= 1 << bitInByte
			}
		}
	}
}
