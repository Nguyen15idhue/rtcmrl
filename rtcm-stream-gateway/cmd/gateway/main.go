package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/capture"
	"github.com/your-org/rtcm-stream-gateway/internal/engine"
)

func main() {
	device := flag.String("device", env("DEVICE", "any"), "capture interface")
	listenPort := flag.Int("listen-port", envInt("LISTEN_PORT", 12101), "incoming TCP port")
	snaplen := flag.Int("snaplen", envInt("SNAPLEN", 262144), "pcap snaplen")
	bufferMB := flag.Int("buffer-mb", envInt("BUFFER_MB", 64), "pcap kernel buffer MB")

	casterHost := flag.String("caster-host", env("CASTER_HOST", ""), "caster destination host")
	casterPort := flag.Int("caster-port", envInt("CASTER_PORT", 2101), "caster destination port")
	casterPass := flag.String("caster-pass", env("CASTER_PASS", ""), "caster SOURCE password")
	mountPrefix := flag.String("mount-prefix", env("MOUNT_PREFIX", "STN"), "mountpoint prefix, final mount is PREFIX_XXXX")

	sourceIdleSec := flag.Int("source-idle-sec", envInt("SOURCE_IDLE_SEC", 20), "remove idle source mapping after N seconds")
	stationIdleSec := flag.Int("station-idle-sec", envInt("STATION_IDLE_SEC", 180), "close idle station output after N seconds")
	statsSec := flag.Int("stats-sec", envInt("STATS_SEC", 5), "stats interval seconds")

	flag.Parse()

	if *casterHost == "" || *casterPass == "" {
		log.Fatal("missing required output params: -caster-host, -caster-pass")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	eng := engine.New(engine.Config{
		Caster: engine.CasterConfig{
			Host:        *casterHost,
			Port:        *casterPort,
			Pass:        *casterPass,
			MountPrefix: *mountPrefix,
		},
		SourceIdle:    time.Duration(*sourceIdleSec) * time.Second,
		StationIdle:   time.Duration(*stationIdleSec) * time.Second,
		StatsInterval: time.Duration(*statsSec) * time.Second,
		QueueSize:     16384,
	})

	go eng.Run(ctx)

	cfg := capture.Config{
		Device:     *device,
		ListenPort: *listenPort,
		SnapLen:    int32(*snaplen),
		BufferMB:   *bufferMB,
	}

	log.Printf("[BOOT] device=%s port=%d out=%s:%d prefix=%s", *device, *listenPort, *casterHost, *casterPort, *mountPrefix)
	if err := capture.Run(ctx, cfg, eng); err != nil {
		log.Fatalf("capture failed: %v", err)
	}
	log.Printf("[STOP] done")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
