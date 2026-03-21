package capture

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/your-org/rtcm-stream-gateway/internal/engine"
	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

type Config struct {
	Device     string
	ListenPort int
	SnapLen    int32
	BufferMB   int
}

type streamFactory struct {
	ctx        context.Context
	eng        *engine.Engine
	listenPort int
}

type stream struct {
	r tcpreader.ReaderStream
	f *streamFactory
}

func (sf *streamFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	s := &stream{r: tcpreader.NewReaderStream(), f: sf}
	go s.run(netFlow, tcpFlow)
	return &s.r
}

func (s *stream) run(netFlow, tcpFlow gopacket.Flow) {
	defer func() {
		_ = recover()
	}()

	srcIP := netFlow.Src().String()
	dstIP := netFlow.Dst().String()
	srcPort := tcpFlow.Src().String()
	dstPort := tcpFlow.Dst().String()

	// Only keep payload into destination port 12101.
	dp, err := strconv.Atoi(dstPort)
	if err != nil || dp != s.f.listenPort {
		_, _ = io.Copy(io.Discard, &s.r)
		return
	}

	id := fmt.Sprintf("%s:%s>%s:%s", srcIP, srcPort, dstIP, dstPort)
	log.Printf("[FLOW] open %s", id)
	defer log.Printf("[FLOW] close %s", id)

	scanner := rtcm.NewScanner()
	br := bufio.NewReaderSize(&s.r, 64*1024)
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-s.f.ctx.Done():
			return
		default:
		}
		n, err := br.Read(buf)
		if n > 0 {
			frames := scanner.Push(buf[:n])
			now := time.Now()
			for _, fr := range frames {
				s.f.eng.Input(engine.InFrame{SourceKey: id, SourceIP: srcIP, Frame: fr, At: now})
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[FLOW] read error %s: %v", id, err)
			}
			return
		}
	}
}

func Run(ctx context.Context, cfg Config, eng *engine.Engine) error {
	inactive, err := pcap.NewInactiveHandle(cfg.Device)
	if err != nil {
		return err
	}
	defer inactive.CleanUp()

	if err := inactive.SetSnapLen(int(cfg.SnapLen)); err != nil {
		return err
	}
	if err := inactive.SetPromisc(true); err != nil {
		return err
	}
	if err := inactive.SetTimeout(pcap.BlockForever); err != nil {
		return err
	}
	if cfg.BufferMB > 0 {
		if err := inactive.SetBufferSize(cfg.BufferMB * 1024 * 1024); err != nil {
			return err
		}
	}

	handle, err := inactive.Activate()
	if err != nil {
		return err
	}
	defer handle.Close()

	filter := fmt.Sprintf("tcp port %d", cfg.ListenPort)
	if err := handle.SetBPFFilter(filter); err != nil {
		return fmt.Errorf("set BPF '%s': %w", filter, err)
	}
	log.Printf("[CAP] device=%s bpf=%q", cfg.Device, filter)

	factory := &streamFactory{ctx: ctx, eng: eng, listenPort: cfg.ListenPort}
	pool := tcpassembly.NewStreamPool(factory)
	assembler := tcpassembly.NewAssembler(pool)
	assembler.MaxBufferedPagesPerConnection = 1024
	assembler.MaxBufferedPagesTotal = 1024 * 1024

	packets := gopacket.NewPacketSource(handle, handle.LinkType())
	packets.Lazy = true
	packets.NoCopy = true

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			assembler.FlushOlderThan(time.Now().Add(-8 * time.Second))
		case p, ok := <-packets.Packets():
			if !ok {
				return nil
			}
			n := p.NetworkLayer()
			t := p.TransportLayer()
			if n == nil || t == nil {
				continue
			}
			tcp, ok := t.(*layers.TCP)
			if !ok {
				continue
			}
			assembler.AssembleWithTimestamp(n.NetworkFlow(), tcp, p.Metadata().Timestamp)
		}
	}
}
