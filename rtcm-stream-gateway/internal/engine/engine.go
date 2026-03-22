package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/your-org/rtcm-stream-gateway/internal/caster"
	"github.com/your-org/rtcm-stream-gateway/internal/metrics"
	"github.com/your-org/rtcm-stream-gateway/internal/rtcm"
)

type InFrame struct {
	SourceKey string
	SourceIP  string
	Frame     []byte
	At        time.Time
}

type CasterConfig struct {
	Host         string
	Port         int
	Pass         string
	MountPrefix  string
	NtripVersion int
	User         string
}

type Config struct {
	Caster        CasterConfig
	SourceIdle    time.Duration
	StationIdle   time.Duration
	StatsInterval time.Duration
	QueueSize     int
	TestMode      bool
}

type sourceState struct {
	SourceKey   string
	SourceIP    string
	StationID   int
	Fingerprint string
	VariantKey  string
	LastSeen    time.Time
	FramesIn    uint64
	BytesIn     uint64
}

type stationState struct {
	ID         int
	VariantKey string
	Mount      string
	LastSeen   time.Time
	FramesOut  uint64
	BytesOut   uint64
	Client     *caster.Client
	Enabled    bool
}

type stationRoot struct {
	ID         int
	PrimaryKey string
	Variants   map[string]*stationState
}

type Engine struct {
	cfg Config

	inCh     chan InFrame
	mu       sync.RWMutex
	testMode bool

	sources  map[string]*sourceState
	stations map[int]*stationRoot

	drops     uint64
	unknown   uint64
	ambiguous uint64
	forwarded uint64
}

func New(cfg Config) *Engine {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 4096
	}
	if cfg.SourceIdle <= 0 {
		cfg.SourceIdle = 20 * time.Second
	}
	if cfg.StationIdle <= 0 {
		cfg.StationIdle = 3 * time.Minute
	}
	if cfg.StatsInterval <= 0 {
		cfg.StatsInterval = 5 * time.Second
	}
	if cfg.Caster.MountPrefix == "" {
		cfg.Caster.MountPrefix = "STN"
	}

	return &Engine{
		cfg:      cfg,
		inCh:     make(chan InFrame, cfg.QueueSize),
		sources:  make(map[string]*sourceState),
		stations: make(map[int]*stationRoot),
		testMode: cfg.TestMode,
	}
}

func (e *Engine) Input(f InFrame) {
	select {
	case e.inCh <- f:
	default:
		e.mu.Lock()
		e.drops++
		e.mu.Unlock()
		metrics.FramesDropped.Inc()
	}
}

func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.closeAllStations()
			return
		case f := <-e.inCh:
			e.onFrame(f)
		case <-ticker.C:
			now := time.Now()
			e.gcSources(now)
			e.gcStations(now)
			e.printStats()
			metrics.ActiveStations.Set(float64(e.stationCount()))
		}
	}
}

func (e *Engine) onFrame(f InFrame) {
	e.mu.Lock()
	s := e.sources[f.SourceKey]
	if s == nil {
		s = &sourceState{SourceKey: f.SourceKey, SourceIP: f.SourceIP}
		e.sources[f.SourceKey] = s
	}
	s.LastSeen = f.At
	s.FramesIn++
	s.BytesIn += uint64(len(f.Frame))
	e.mu.Unlock()

	stationID, ok := rtcm.StationID(f.Frame)
	if !ok {
		e.mu.Lock()
		e.unknown++
		e.mu.Unlock()
		metrics.FramesUnknown.Inc()
		return
	}

	if fp, ok := rtcm.StationFingerprint(f.Frame); ok {
		e.mu.Lock()
		s := e.sources[f.SourceKey]
		if s != nil {
			s.Fingerprint = fp
		}
		e.mu.Unlock()
	}

	st, sendOK := e.routeForSource(f.SourceKey, stationID, f.At)
	if !sendOK {
		e.mu.Lock()
		e.ambiguous++
		e.mu.Unlock()
		metrics.FramesAmbiguous.Inc()
		return
	}

	if !st.Enabled {
		return
	}

	if !e.testMode && st.Client != nil {
		if err := st.Client.Send(f.Frame); err != nil {
			log.Printf("[WARN] station=%d mount=%s send failed: %v", st.ID, st.Mount, err)
			return
		}
	}

	e.mu.Lock()
	st.LastSeen = f.At
	st.FramesOut++
	st.BytesOut += uint64(len(f.Frame))
	e.forwarded++
	e.mu.Unlock()

	metrics.FramesForwarded.Inc()
	metrics.BytesForwarded.Add(float64(len(f.Frame)))
}

func (e *Engine) routeForSource(sourceKey string, stationID int, now time.Time) (*stationState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.sources[sourceKey]
	if s == nil {
		e.unknown++
		return nil, false
	}
	s.StationID = stationID

	root := e.stations[stationID]
	if root == nil {
		root = &stationRoot{ID: stationID, Variants: make(map[string]*stationState)}
		e.stations[stationID] = root
	}

	variantKey := sourceKey

	s.VariantKey = variantKey

	if st := root.Variants[variantKey]; st != nil {
		return st, true
	}

	mount := e.mountForVariant(root, variantKey)

	var client *caster.Client
	if !e.testMode {
		if e.cfg.Caster.NtripVersion == 2 && e.cfg.Caster.User != "" {
			client = caster.NewWithAuth(e.cfg.Caster.Host, e.cfg.Caster.Port, e.cfg.Caster.User, e.cfg.Caster.Pass, mount, e.cfg.Caster.NtripVersion)
		} else {
			client = caster.New(e.cfg.Caster.Host, e.cfg.Caster.Port, e.cfg.Caster.Pass, mount)
		}
	}

	st := &stationState{
		ID:         stationID,
		VariantKey: variantKey,
		Mount:      mount,
		Client:     client,
		LastSeen:   now,
		Enabled:    true,
	}

	root.Variants[variantKey] = st
	if root.PrimaryKey == "" {
		root.PrimaryKey = variantKey
	}
	log.Printf("[NEW] station=%d variant=%s mount=%s", stationID, short(variantKey), mount)
	return st, true
}

func (e *Engine) mountForVariant(root *stationRoot, variantKey string) string {
	return fmt.Sprintf("%s_%04d", e.cfg.Caster.MountPrefix, root.ID)
}

func short(s string) string {
	if len(s) <= 6 {
		return s
	}
	return s[:6]
}

func (e *Engine) gcSources(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for k, s := range e.sources {
		if now.Sub(s.LastSeen) > e.cfg.SourceIdle {
			delete(e.sources, k)
		}
	}
}

func (e *Engine) gcStations(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for stationID, root := range e.stations {
		for vk, st := range root.Variants {
			if now.Sub(st.LastSeen) > e.cfg.StationIdle {
				if !e.testMode && st.Client != nil {
					_ = st.Client.Close()
				}
				delete(root.Variants, vk)
				if root.PrimaryKey == vk {
					root.PrimaryKey = ""
				}
			}
		}
		if len(root.Variants) == 0 {
			delete(e.stations, stationID)
		}
	}
}

func (e *Engine) closeAllStations() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.testMode {
		return
	}
	for _, root := range e.stations {
		for _, st := range root.Variants {
			if st.Client != nil {
				_ = st.Client.Close()
			}
		}
	}
}

func (e *Engine) printStats() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	log.Printf("[STAT] sources=%d stations=%d forwarded=%d unknown=%d ambiguous=%d drops=%d",
		len(e.sources), len(e.stations), e.forwarded, e.unknown, e.ambiguous, e.drops)

	for sid, root := range e.stations {
		for _, st := range root.Variants {
			log.Printf("[MOUNT] station=%d mount=%s variant=%s out_frames=%d out_bytes=%d last=%s",
				sid, st.Mount, short(st.VariantKey), st.FramesOut, st.BytesOut, st.LastSeen.Format(time.RFC3339))
		}
	}
}

func (e *Engine) stationCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.stations)
}

func (e *Engine) GetStations() []map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(e.stations))
	for sid, root := range e.stations {
		for vk, st := range root.Variants {
			result = append(result, map[string]interface{}{
				"station_id":  sid,
				"variant_key": vk,
				"mount":       st.Mount,
				"enabled":     st.Enabled,
				"last_seen":   st.LastSeen.Format(time.RFC3339),
				"frames_out":  st.FramesOut,
				"bytes_out":   st.BytesOut,
				"source_ip":   e.sourceIPForStation(sid, vk),
			})
		}
	}
	return result
}

func (e *Engine) sourceIPForStation(stationID int, variantKey string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, s := range e.sources {
		if s.StationID == stationID && s.VariantKey == variantKey {
			return s.SourceIP
		}
	}
	return ""
}

func (e *Engine) GetStationByID(id int) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	root := e.stations[id]
	if root == nil {
		return nil
	}

	variants := make([]map[string]interface{}, 0, len(root.Variants))
	for vk, st := range root.Variants {
		variants = append(variants, map[string]interface{}{
			"variant_key": vk,
			"mount":       st.Mount,
			"enabled":     st.Enabled,
			"last_seen":   st.LastSeen.Format(time.RFC3339),
			"frames_out":  st.FramesOut,
			"bytes_out":   st.BytesOut,
		})
	}
	return map[string]interface{}{
		"station_id": id,
		"variants":   variants,
	}
}

func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]interface{}{
		"sources":   len(e.sources),
		"stations":  len(e.stations),
		"forwarded": e.forwarded,
		"unknown":   e.unknown,
		"ambiguous": e.ambiguous,
		"drops":     e.drops,
	}
}

func (e *Engine) UpdateRuntimeConfig(cfg struct {
	SourceIdle    time.Duration
	StationIdle   time.Duration
	StatsInterval time.Duration
}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.SourceIdle = cfg.SourceIdle
	e.cfg.StationIdle = cfg.StationIdle
	e.cfg.StatsInterval = cfg.StatsInterval
}

func (e *Engine) EnableStation(stationID int, enabled bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	root := e.stations[stationID]
	if root == nil {
		return false
	}
	for _, st := range root.Variants {
		st.Enabled = enabled
	}
	return true
}

func (e *Engine) SetStationEnabled(stationID int, variantKey string, enabled bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	root := e.stations[stationID]
	if root == nil {
		return false
	}
	st := root.Variants[variantKey]
	if st == nil {
		return false
	}
	st.Enabled = enabled
	return true
}
