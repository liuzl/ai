package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector manages Prometheus metrics for the proxy
type MetricsCollector struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	errorsTotal     *prometheus.CounterVec
	activeRequests  *prometheus.GaugeVec
}

// NewMetricsCollector creates a new MetricsCollector and registers all metrics
func NewMetricsCollector() *MetricsCollector {
	m := &MetricsCollector{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_requests_total",
				Help: "Total number of proxy requests",
			},
			[]string{"format", "model", "provider", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "proxy_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
			},
			[]string{"format", "model", "provider"},
		),
		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_errors_total",
				Help: "Total number of errors",
			},
			[]string{"format", "model", "provider", "error_type"},
		),
		activeRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "proxy_active_requests",
				Help: "Number of active requests",
			},
			[]string{"format", "provider"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(m.requestsTotal)
	prometheus.MustRegister(m.requestDuration)
	prometheus.MustRegister(m.errorsTotal)
	prometheus.MustRegister(m.activeRequests)

	return m
}

// RecordRequest records a completed request
func (m *MetricsCollector) RecordRequest(format, model, provider, status string, duration time.Duration) {
	m.requestsTotal.WithLabelValues(format, model, provider, status).Inc()
	m.requestDuration.WithLabelValues(format, model, provider).Observe(duration.Seconds())
}

// RecordError records an error
func (m *MetricsCollector) RecordError(format, model, provider, errorType string) {
	m.errorsTotal.WithLabelValues(format, model, provider, errorType).Inc()
}

// IncActiveRequests increments the active request counter
func (m *MetricsCollector) IncActiveRequests(format, provider string) {
	m.activeRequests.WithLabelValues(format, provider).Inc()
}

// DecActiveRequests decrements the active request counter
func (m *MetricsCollector) DecActiveRequests(format, provider string) {
	m.activeRequests.WithLabelValues(format, provider).Dec()
}

// Handler returns an HTTP handler for the /metrics endpoint
func (m *MetricsCollector) Handler() http.Handler {
	return promhttp.Handler()
}
