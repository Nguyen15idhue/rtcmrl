package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/your-org/rtcm-stream-gateway/internal/config"
	"github.com/your-org/rtcm-stream-gateway/internal/engine"
	"github.com/your-org/rtcm-stream-gateway/internal/generator"
	"github.com/your-org/rtcm-stream-gateway/internal/worker"
)

type Server struct {
	cfgManager *config.Manager
	eng        *engine.Engine
	workerPool *worker.Pool
	gen        *generator.Generator
	router     *chi.Mux
	httpSrv    *http.Server
	metricsSrv *http.Server
	startTime  time.Time
	mode       string
	device     string
}

func New(cfgManager *config.Manager, eng *engine.Engine, pool *worker.Pool, webPort, metricsPort int) *Server {
	r := chi.NewRouter()
	cfg := cfgManager.Get()
	s := &Server{
		cfgManager: cfgManager,
		eng:        eng,
		workerPool: pool,
		router:     r,
		startTime:  time.Now(),
		mode:       cfg.Mode,
		device:     cfg.Capture.Device,
	}

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	s.setupRoutes(r)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", webPort),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.metricsSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsPort),
		Handler: promhttp.Handler(),
	}

	return s
}

func (s *Server) setupRoutes(r *chi.Mux) {
	cfg := s.cfgManager.Get()

	frontendRoot := cfg.Web.WebRoot
	if frontendRoot == "" {
		frontendRoot = "frontend"
	}

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", s.handleHealth)
		api.Get("/stations", s.handleStations)
		api.Get("/stations/{id}", s.handleStationByID)
		api.Get("/stations/quality", s.handleAllStationQuality)
		api.Get("/stations/{id}/quality", s.handleStationQuality)
		api.Delete("/stations", s.handleCleanupStations)
		api.Get("/stats", s.handleStats)
		api.Get("/config", s.handleGetConfig)
		api.Post("/config", s.handleUpdateConfig)
		api.Get("/workers", s.handleWorkers)
		api.Post("/workers", s.handleSetWorkers)
		api.Post("/workers/auto-scale", s.handleSetAutoScale)
		api.Post("/restart", s.handleRestart)

		// Generator API
		api.Get("/generator", s.handleGeneratorGet)
		api.Post("/generator/start", s.handleGeneratorStart)
		api.Post("/generator/stop", s.handleGeneratorStop)

		// Mode API
		api.Get("/mode", s.handleGetMode)
		api.Post("/mode", s.handleSetMode)
		api.Get("/mode/test", s.handleTestCapture)
		api.Get("/network", s.handleNetworkInfo)
	})

	if _, err := os.Stat(frontendRoot); err == nil {
		frontendRootAbs, _ := filepath.Abs(frontendRoot)
		distDir := frontendRootAbs + "/dist"

		if _, err := os.Stat(distDir); err == nil {
			staticHandler := http.StripPrefix("/assets/", http.FileServer(http.Dir(distDir+"/assets")))
			r.Get("/assets/{file}", staticHandler.ServeHTTP)
			r.Get("/assets/*", staticHandler.ServeHTTP)
		}

		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			indexPath := frontendRootAbs + "/dist/index.html"
			if _, err := os.Stat(indexPath); err != nil {
				indexPath = frontendRootAbs + "/index.html"
			}
			http.ServeFile(w, req, indexPath)
		})
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/assets/") {
				http.NotFound(w, req)
				return
			}
			http.ServeFile(w, req, frontendRootAbs+"/dist/index.html")
		})
	} else {
		r.Get("/", s.serveIndex)
	}

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/debug/pprof/*", func(w http.ResponseWriter, r *http.Request) {
		http.DefaultServeMux.ServeHTTP(w, r)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgManager.Get()
	if cfg.Web.WebRoot != "" {
		indexPath := cfg.Web.WebRoot + "/index.html"
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
	}

	data, _ := os.ReadFile("frontend/dist/index.html")
	if len(data) > 0 {
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service": "rtcm-stream-gateway",
		"version": "2.0.0",
		"status":  "running",
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := "healthy"
	queueFill := float64(s.workerPool.QueueSize()) / 4096.0
	if queueFill > 0.9 {
		status = "degraded"
	}

	resp := map[string]interface{}{
		"status":     status,
		"uptime":     time.Since(s.startTime).String(),
		"goroutine":  runtime.NumGoroutine(),
		"mem_mb":     m.Alloc / 1024 / 1024,
		"queue_fill": queueFill,
	}

	w.Header().Set("Content-Type", "application/json")
	if status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStations(w http.ResponseWriter, r *http.Request) {
	stations := s.eng.GetStations()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":    len(stations),
		"stations": stations,
	})
}

func (s *Server) handleStationByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid station id", http.StatusBadRequest)
		return
	}

	station := s.eng.GetStationByID(id)
	if station == nil {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.eng.GetStats()
	stats["queue_depth"] = s.workerPool.QueueSize()
	stats["workers_active"] = s.workerPool.ActiveWorkers()
	stats["workers_desired"] = s.workerPool.DesiredWorkers()
	stats["uptime"] = time.Since(s.startTime).String()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats["mem_alloc_mb"] = m.Alloc / 1024 / 1024
	stats["goroutines"] = runtime.NumGoroutine()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgManager.Get()
	resp := map[string]interface{}{
		"capture":    cfg.Capture,
		"caster":     cfg.Caster,
		"web":        cfg.Web,
		"worker":     cfg.Worker,
		"runtime":    cfg.Runtime,
		"auto_scale": cfg.Worker.AutoScale,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	cfg := s.cfgManager.Get()
	if workerCfg, ok := updates["worker"].(map[string]interface{}); ok {
		if v, ok := workerCfg["auto_scale"].(bool); ok {
			cfg.Worker.AutoScale = v
			s.workerPool.SetAutoScale(v)
		}
		if v, ok := workerCfg["min"].(float64); ok {
			cfg.Worker.Min = int(v)
		}
		if v, ok := workerCfg["max"].(float64); ok {
			cfg.Worker.Max = int(v)
		}
	}

	if rtCfg, ok := updates["runtime"].(map[string]interface{}); ok {
		if v, ok := rtCfg["source_idle_sec"].(float64); ok {
			cfg.Runtime.SourceIdle = time.Duration(v) * time.Second
		}
		if v, ok := rtCfg["station_idle_sec"].(float64); ok {
			cfg.Runtime.StationIdle = time.Duration(v) * time.Second
		}
	}

	if casterCfg, ok := updates["caster"].(map[string]interface{}); ok {
		if v, ok := casterCfg["host"].(string); ok {
			cfg.Caster.Host = v
		}
		if v, ok := casterCfg["port"].(float64); ok {
			cfg.Caster.Port = int(v)
		}
		if v, ok := casterCfg["mount_prefix"].(string); ok {
			cfg.Caster.MountPrefix = v
		}
		if v, ok := casterCfg["pass"].(string); ok {
			cfg.Caster.Pass = v
		}
		if v, ok := casterCfg["ntrip_version"].(float64); ok {
			cfg.Caster.NtripVersion = int(v)
		}
		if v, ok := casterCfg["user"].(string); ok {
			cfg.Caster.User = v
		}
	}

	if captureCfg, ok := updates["capture"].(map[string]interface{}); ok {
		if v, ok := captureCfg["listen_port"].(float64); ok {
			cfg.Capture.ListenPort = int(v)
		}
		if v, ok := captureCfg["device"].(string); ok {
			cfg.Capture.Device = v
		}
	}

	s.cfgManager.Update(cfg)
	if err := s.cfgManager.Save(); err != nil {
		log.Printf("[WARN] config save failed: %v", err)
	}

	s.eng.UpdateRuntimeConfig(struct {
		SourceIdle    time.Duration
		StationIdle   time.Duration
		StatsInterval time.Duration
	}{
		SourceIdle:    cfg.Runtime.SourceIdle,
		StationIdle:   cfg.Runtime.StationIdle,
		StatsInterval: cfg.Runtime.StatsInterval,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgManager.Get()
	resp := map[string]interface{}{
		"active":  s.workerPool.ActiveWorkers(),
		"desired": s.workerPool.DesiredWorkers(),
		"min":     cfg.Worker.Min,
		"max":     cfg.Worker.Max,
		"auto":    cfg.Worker.AutoScale,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSetWorkers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	cfg := s.cfgManager.Get()
	if req.Count < cfg.Worker.Min || req.Count > cfg.Worker.Max {
		http.Error(w, fmt.Sprintf("count must be between %d and %d", cfg.Worker.Min, cfg.Worker.Max), http.StatusBadRequest)
		return
	}

	s.workerPool.SetDesiredWorkers(req.Count)
	cfg = s.cfgManager.Get()
	cfg.Worker.Min = req.Count
	s.cfgManager.Update(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"desired": req.Count})
}

func (s *Server) handleSetAutoScale(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	s.workerPool.SetAutoScale(req.Enabled)
	cfg := s.cfgManager.Get()
	cfg.Worker.AutoScale = req.Enabled
	s.cfgManager.Update(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"auto_scale": req.Enabled})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func (s *Server) Start(ctx context.Context) {
	go func() {
		log.Printf("[WEB] HTTP server starting on %s", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[WEB] HTTP server error: %v", err)
		}
	}()

	go func() {
		log.Printf("[MET] Metrics server starting on %s", s.metricsSrv.Addr)
		if err := s.metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[MET] Metrics server error: %v", err)
		}
	}()

	<-ctx.Done()
	s.shutdown()
}

func (s *Server) shutdown() {
	log.Printf("[WEB] shutting down HTTP servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WEB] HTTP shutdown error: %v", err)
	}
	if err := s.metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[MET] metrics shutdown error: %v", err)
	}
}

func (s *Server) handleStationQuality(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid station id", http.StatusBadRequest)
		return
	}

	quality := s.eng.GetStationQuality(id)
	if quality == nil {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quality)
}

func (s *Server) handleAllStationQuality(w http.ResponseWriter, r *http.Request) {
	qualities := s.eng.GetAllStationQuality()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(qualities)
}

func (s *Server) handleGeneratorGet(w http.ResponseWriter, r *http.Request) {
	if s.gen == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"running": false,
			"config":  nil,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running": s.gen.IsRunning(),
		"config":  s.gen.GetConfig(),
	})
}

func (s *Server) handleGeneratorStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host       string `json:"host"`
		Stations   []int  `json:"stations"`
		IntervalMs int    `json:"interval_ms"`
		FrameType  uint16 `json:"frame_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.IntervalMs <= 0 {
		req.IntervalMs = 1000
	}
	if req.FrameType == 0 {
		req.FrameType = 1006
	}

	cfg := generator.Config{
		Host:       req.Host,
		Port:       12101,
		StationIDs: req.Stations,
		IntervalMs: req.IntervalMs,
		FrameType:  req.FrameType,
	}

	if s.gen == nil {
		s.gen = generator.New(cfg)
	} else {
		s.gen.SetConfig(cfg)
	}

	if err := s.gen.Start(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running": true,
		"config":  s.gen.GetConfig(),
	})
}

func (s *Server) handleGeneratorStop(w http.ResponseWriter, r *http.Request) {
	if s.gen != nil {
		s.gen.Stop()
	}

	s.eng.CleanupAllStations()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running": false,
	})
}

func (s *Server) handleCleanupStations(w http.ResponseWriter, r *http.Request) {
	count := s.eng.CleanupAllStations()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"cleanup": count})
}

func (s *Server) handleGetMode(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgManager.Get()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":   s.mode,
		"device": s.device,
		"port":   cfg.Capture.ListenPort,
		"config": map[string]interface{}{
			"mode":   cfg.Mode,
			"device": cfg.Capture.Device,
		},
	})
}

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode   string `json:"mode"`
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Mode != "tcp" && req.Mode != "pcap" && req.Mode != "auto" && req.Mode != "sniff" {
		http.Error(w, "mode must be tcp, pcap, sniff, or auto", http.StatusBadRequest)
		return
	}

	s.mode = req.Mode
	s.device = req.Device

	cfg := s.cfgManager.Get()
	cfg.Mode = req.Mode
	if req.Device != "" {
		cfg.Capture.Device = req.Device
	}
	s.cfgManager.Update(cfg)
	s.cfgManager.Save()

	log.Printf("[MODE] mode changed to: %s, device: %s", req.Mode, req.Device)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":    s.mode,
		"device":  s.device,
		"message": "Mode changed. Restart gateway to apply.",
	})
}

func (s *Server) handleTestCapture(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfgManager.Get()
	result := map[string]interface{}{
		"mode":           s.mode,
		"device":         s.device,
		"port":           cfg.Capture.ListenPort,
		"port_listening": false,
		"pcap_available": false,
	}

	if addr := fmt.Sprintf(":%d", cfg.Capture.ListenPort); addr != ":" {
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			result["port_listening"] = false
		} else {
			result["port_listening"] = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleNetworkInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"hostname":   getHostname(),
		"platform":   runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
