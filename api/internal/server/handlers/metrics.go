package handlers

import (
	"net/http"

	"github.com/aegisrun/aegisrun/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsHandler struct {
	registry *telemetry.MetricsRegistry
}

func NewMetricsHandler(registry *telemetry.MetricsRegistry) *MetricsHandler {
	return &MetricsHandler{registry: registry}
}

func (h *MetricsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
