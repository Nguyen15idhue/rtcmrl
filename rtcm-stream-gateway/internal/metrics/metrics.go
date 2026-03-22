package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	FramesForwarded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rtcm_frames_forwarded_total",
		Help: "Total frames successfully forwarded",
	})

	FramesDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rtcm_frames_dropped_total",
		Help: "Total frames dropped (queue full)",
	})

	FramesUnknown = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rtcm_frames_unknown_total",
		Help: "Total frames with no StationID",
	})

	FramesAmbiguous = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rtcm_frames_ambiguous_total",
		Help: "Total frames with ambiguous StationID",
	})

	BytesForwarded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rtcm_bytes_forwarded_total",
		Help: "Total bytes forwarded",
	})

	ActiveStations = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_stations_active",
		Help: "Number of active stations",
	})

	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_workers_active",
		Help: "Number of active workers",
	})

	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_queue_depth",
		Help: "Current frame queue depth",
	})

	QueueCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_queue_capacity",
		Help: "Frame queue capacity",
	})

	ConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_connections_active",
		Help: "Number of active TCP connections to caster",
	})

	CpuUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rtcm_cpu_usage",
		Help: "Estimated CPU usage (0-1)",
	})
)
