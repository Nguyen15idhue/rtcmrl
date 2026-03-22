package capture

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

type PcapConfig struct {
	Interface  string
	ListenPort int
	QueueSize  int
	SnapLen    int
	Promisc    bool
	Timeout    time.Duration
}

type PcapCapture struct {
	cfg     PcapConfig
	handler FrameHandler
}

func NewPcapCapture(cfg PcapConfig, handler FrameHandler) *PcapCapture {
	return &PcapCapture{cfg: cfg, handler: handler}
}

func DetectBestMode(iface string, port int) string {
	if iface == "auto" || iface == "" {
		iface = "any"
	}

	if iface == "any" {
		devs, err := pcap.FindAllDevs()
		if err != nil {
			log.Printf("[AUTO] pcap not available: %v", err)
			return "tcp"
		}

		if len(devs) == 0 {
			log.Printf("[AUTO] no network devices found")
			return "tcp"
		}

		handle, err := pcap.OpenLive(devs[0].Name, 1600, false, 100*time.Millisecond)
		if err != nil {
			log.Printf("[AUTO] cannot open pcap device: %v", err)
			return "tcp"
		}
		handle.Close()

		log.Printf("[AUTO] pcap available, using sniff mode (no port bind)")
		return "pcap"
	}

	handle, err := pcap.OpenLive(iface, 1600, false, 100*time.Millisecond)
	if err != nil {
		log.Printf("[AUTO] cannot open pcap on %s: %v", iface, err)
		return "tcp"
	}
	handle.Close()

	log.Printf("[AUTO] pcap available on %s, using sniff mode", iface)
	return "pcap"
}

func (p *PcapCapture) Run(ctx context.Context) error {
	var iface string
	if p.cfg.Interface == "" || p.cfg.Interface == "any" {
		iface = "any"
	} else {
		iface = p.cfg.Interface
	}

	snapLen := p.cfg.SnapLen
	if snapLen <= 0 {
		snapLen = 1600
	}

	var handle *pcap.Handle
	var err error

	if iface == "any" {
		devs, err := pcap.FindAllDevs()
		if err != nil {
			return fmt.Errorf("find devices: %v", err)
		}

		for _, dev := range devs {
			log.Printf("[PCAP] device: %s, addrs: %v", dev.Name, dev.Addresses)
		}

		if len(devs) == 0 {
			return fmt.Errorf("no network devices found")
		}

		handle, err = pcap.OpenLive(devs[0].Name, int32(snapLen), true, p.cfg.Timeout)
	} else {
		handle, err = pcap.OpenLive(iface, int32(snapLen), true, p.cfg.Timeout)
	}

	if err != nil {
		return fmt.Errorf("open %s: %v", iface, err)
	}

	filter := fmt.Sprintf("tcp port %d", p.cfg.ListenPort)
	if err := handle.SetBPFFilter(filter); err != nil {
		handle.Close()
		return fmt.Errorf("set filter '%s': %v", filter, err)
	}

	log.Printf("[PCAP] capturing on %s, filter: %s", iface, filter)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.Lazy = true
	packetSource.NoCopy = true

	for {
		select {
		case <-ctx.Done():
			handle.Close()
			return nil
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}
			p.processPacket(packet)
		}
	}
}

func (p *PcapCapture) processPacket(packet gopacket.Packet) {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	ip, _ := ipLayer.(*layers.IPv4)

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp, _ := tcpLayer.(*layers.TCP)

	srcIP := ip.SrcIP.String()
	dstIP := ip.DstIP.String()
	srcPort := int(tcp.SrcPort)
	dstPort := int(tcp.DstPort)

	payload := tcp.Payload
	if len(payload) == 0 {
		return
	}

	if dstPort != p.cfg.ListenPort && srcPort != p.cfg.ListenPort {
		return
	}

	sourceKey := fmt.Sprintf("%s:%d", srcIP, srcPort)
	if dstPort == p.cfg.ListenPort {
		sourceKey = fmt.Sprintf("%s:%d", dstIP, dstPort)
	}

	now := time.Now()
	scanner := rtcm.NewScanner()
	frames := scanner.Push(payload)
	for _, frame := range frames {
		p.handler(sourceKey, srcIP, frame, now)
	}
}
