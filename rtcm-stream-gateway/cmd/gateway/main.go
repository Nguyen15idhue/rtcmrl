package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/capture"
	"github.com/your-org/rtcm-stream-gateway/internal/config"
	"github.com/your-org/rtcm-stream-gateway/internal/engine"
	"github.com/your-org/rtcm-stream-gateway/internal/web"
	"github.com/your-org/rtcm-stream-gateway/internal/worker"
)

func main() {
	cfgManager := config.New()

	// Set config file path
	if fp := os.Getenv("CONFIG_FILE"); fp != "" {
		cfgManager.SetFilePath(fp)
		log.Printf("[BOOT] config file: %s", fp)
	}

	cfg := cfgManager.Get()

	mode := os.Getenv("CAPTURE_MODE")
	if mode == "" {
		mode = os.Getenv("MODE")
	}

	testMode := cfg.Caster.Host == "test" || os.Getenv("TEST_MODE") == "1"

	if !testMode && (cfg.Caster.Host == "" || cfg.Caster.Pass == "") {
		log.Fatal("missing required: CASTER_HOST and CASTER_PASS must be set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[BOOT] shutdown signal received")
		cancel()
	}()

	eng := engine.New(engine.Config{
		Caster: engine.CasterConfig{
			Host:         cfg.Caster.Host,
			Port:         cfg.Caster.Port,
			Pass:         cfg.Caster.Pass,
			MountPrefix:  cfg.Caster.MountPrefix,
			NtripVersion: cfg.Caster.NtripVersion,
			User:         cfg.Caster.User,
		},
		SourceIdle:    cfg.Runtime.SourceIdle,
		StationIdle:   cfg.Runtime.StationIdle,
		StatsInterval: cfg.Runtime.StatsInterval,
		QueueSize:     cfg.Worker.QueueSize,
		TestMode:      testMode,
	})

	pool := worker.NewPool(ctx, worker.PoolConfig{
		Min:             cfg.Worker.Min,
		Max:             cfg.Worker.Max,
		QueueSize:       cfg.Worker.QueueSize,
		AutoScale:       cfg.Worker.AutoScale,
		ScaleUpThresh:   cfg.Worker.ScaleUpThresh,
		ScaleDownThresh: cfg.Worker.ScaleDownThresh,
		ScaleInterval:   cfg.Worker.ScaleInterval,
	}, eng)

	go eng.Run(ctx)

	go pool.Start(ctx)

	go func() {
		<-ctx.Done()
		pool.Stop()
	}()

	srv := web.New(cfgManager, eng, pool, cfg.Web.Port, cfg.Web.MetricsPort)
	go srv.Start(ctx)

	log.Printf("[BOOT] rtcm-stream-gateway v2.0.0 starting")
	log.Printf("[BOOT] mode: %s", mode)
	log.Printf("[BOOT] caster: %s:%d prefix=%s", cfg.Caster.Host, cfg.Caster.Port, cfg.Caster.MountPrefix)
	log.Printf("[BOOT] web: :%d metrics: :%d", cfg.Web.Port, cfg.Web.MetricsPort)
	log.Printf("[BOOT] workers: min=%d max=%d auto_scale=%v", cfg.Worker.Min, cfg.Worker.Max, cfg.Worker.AutoScale)

	if mode == "tcp" {
		log.Printf("[BOOT] capture: TCP mode on port %d (no libpcap)", cfg.Capture.ListenPort)
		tcpCfg := capture.TCPConfig{
			ListenPort: cfg.Capture.ListenPort,
			QueueSize:  cfg.Worker.QueueSize,
		}
		handler := func(sourceKey, sourceIP string, frame []byte, at time.Time) {
			pool.Input(engine.InFrame{SourceKey: sourceKey, SourceIP: sourceIP, Frame: frame, At: at})
		}
		listener := capture.NewTCPListener(tcpCfg, handler)
		listener.Run(ctx)
	} else {
		log.Printf("[BOOT] capture: pcap device=%s port=%d", cfg.Capture.Device, cfg.Capture.ListenPort)
		capCfg := capture.Config{
			Device:     cfg.Capture.Device,
			ListenPort: cfg.Capture.ListenPort,
			SnapLen:    int32(cfg.Capture.SnapLen),
			BufferMB:   cfg.Capture.BufferMB,
		}
		if err := capture.Run(ctx, capCfg, eng); err != nil {
			log.Fatalf("[BOOT] capture failed: %v", err)
		}
	}

	log.Printf("[STOP] done")
}
