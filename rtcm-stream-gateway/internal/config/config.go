package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type AppConfig struct {
	Capture CaptureConfig `json:"capture"`
	Caster  CasterConfig  `json:"caster"`
	Web     WebConfig     `json:"web"`
	Worker  WorkerConfig  `json:"worker"`
	Runtime RuntimeConfig `json:"runtime"`
	Mode    string        `json:"mode"`
}

type CaptureConfig struct {
	Device     string `json:"device"`
	ListenPort int    `json:"listen_port"`
	SnapLen    int    `json:"snap_len"`
	BufferMB   int    `json:"buffer_mb"`
}

type CasterConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Pass         string `json:"pass"`
	MountPrefix  string `json:"mount_prefix"`
	NtripVersion int    `json:"ntrip_version"`
	User         string `json:"user"`
}

type WebConfig struct {
	Port        int    `json:"port"`
	MetricsPort int    `json:"metrics_port"`
	WebRoot     string `json:"web_root"`
}

type WorkerConfig struct {
	Min             int           `json:"min"`
	Max             int           `json:"max"`
	QueueSize       int           `json:"queue_size"`
	AutoScale       bool          `json:"auto_scale"`
	ScaleUpThresh   float64       `json:"scale_up_thresh"`
	ScaleDownThresh float64       `json:"scale_down_thresh"`
	ScaleInterval   time.Duration `json:"scale_interval"`
}

type RuntimeConfig struct {
	SourceIdle    time.Duration `json:"source_idle"`
	StationIdle   time.Duration `json:"station_idle"`
	StatsInterval time.Duration `json:"stats_interval"`
}

type Manager struct {
	cfg      *AppConfig
	mu       sync.RWMutex
	filePath string
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func New() *Manager {
	cfg := &AppConfig{
		Caster: CasterConfig{
			Host:         env("CASTER_HOST", "127.0.0.1"),
			Port:         envInt("CASTER_PORT", 2101),
			Pass:         env("CASTER_PASS", "password"),
			MountPrefix:  env("CASTER_MOUNT_PREFIX", "STN"),
			NtripVersion: envInt("NTRIP_VERSION", 2),
			User:         env("CASTER_USER", ""),
		},
		Capture: CaptureConfig{
			Device:     env("DEVICE", "any"),
			ListenPort: envInt("LISTEN_PORT", 12101),
			SnapLen:    envInt("SNAP_LEN", 1024),
			BufferMB:   envInt("BUFFER_MB", 8),
		},
		Worker: WorkerConfig{
			Min:             envInt("WORKER_MIN", 4),
			Max:             envInt("WORKER_MAX", 16),
			QueueSize:       envInt("QUEUE_SIZE", 4096),
			AutoScale:       env("AUTO_SCALE", "true") == "true",
			ScaleUpThresh:   0.8,
			ScaleDownThresh: 0.3,
			ScaleInterval:   10 * time.Second,
		},
		Web: WebConfig{
			Port:        envInt("WEB_PORT", 8080),
			MetricsPort: envInt("METRICS_PORT", 6060),
			WebRoot:     env("WEB_ROOT", ""),
		},
		Runtime: RuntimeConfig{
			SourceIdle:    time.Duration(envInt("SOURCE_IDLE_SEC", 90)) * time.Second,
			StationIdle:   time.Duration(envInt("STATION_IDLE_SEC", 600)) * time.Second,
			StatsInterval: 5 * time.Second,
		},
	}

	m := &Manager{cfg: cfg}

	if fp := os.Getenv("CONFIG_FILE"); fp != "" {
		m.filePath = fp
		if data, err := os.ReadFile(fp); err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				log.Printf("[CFG] config file parse error: %v", err)
			}
		}
	}

	return m
}

func (m *Manager) SetFilePath(fp string) {
	m.filePath = fp
}

func (m *Manager) Get() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Update(cfg *AppConfig) {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()

	// Auto-save to file
	if m.filePath != "" {
		if dir := filepath.Dir(m.filePath); dir != "." && dir != "" {
			os.MkdirAll(dir, 0755)
		}
		if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
			os.WriteFile(m.filePath, data, 0644)
		}
	}
}

func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.filePath == "" {
		return nil
	}

	if dir := filepath.Dir(m.filePath); dir != "." && dir != "" {
		os.MkdirAll(dir, 0755)
	}

	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, m.cfg)
}
