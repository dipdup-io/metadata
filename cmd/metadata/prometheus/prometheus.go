package prometheus

import (
	"strconv"

	golibConfig "github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/resolver"
)

// metric names
const (
	MetricMetadataCounter           = "metadata_counter"
	MetricMetadataNew               = "metadata_new"
	MetricsMetadataHttpErrors       = "metadata_http_errors"
	MetricsMetadataIPFSResponseTime = "metadata_ipfs_response_time"
	MetricsMetadataMimeType         = "metadata_mime_type"
)

// metadata types
const (
	MetadataTypeToken    = "token"
	MetadataTypeContract = "contract"
)

// Prometheus -
type Prometheus struct {
	service *prometheus.Service
}

// NewPrometheus -
func NewPrometheus(cfg *golibConfig.Prometheus) *Prometheus {
	if cfg == nil {
		return nil
	}

	prometheusService := prometheus.NewService(cfg)

	prometheusService.RegisterGoBuildMetrics()
	prometheusService.RegisterGauge(MetricMetadataNew, "Count of new metadata", "type", "network")
	prometheusService.RegisterCounter(MetricMetadataCounter, "Count of metadata", "type", "status", "network")
	prometheusService.RegisterCounter(MetricsMetadataHttpErrors, "Count of HTTP errors in metadata", "network", "code", "type")
	prometheusService.RegisterHistogram(MetricsMetadataIPFSResponseTime, "Histogram showing received bytes from IPFS per millisecons", "network", "node")
	prometheusService.RegisterCounter(MetricsMetadataMimeType, "Count of metadata mime types", "network", "mime")

	return &Prometheus{prometheusService}
}

// Start -
func (p *Prometheus) Start() {
	p.service.Start()
}

// Close -
func (p *Prometheus) Close() error {
	return p.service.Close()
}

// IncrementMetadataNew -
func (p *Prometheus) IncrementMetadataNew(network, typ string) {
	if p.service == nil {
		return
	}
	p.service.IncGaugeValue(MetricMetadataNew, map[string]string{
		"network": network,
		"type":    typ,
	})
}

// DecrementMetadataNew -
func (p *Prometheus) DecrementMetadataNew(network, typ string) {
	if p.service == nil {
		return
	}
	p.service.DecGaugeValue(MetricMetadataNew, map[string]string{
		"network": network,
		"type":    typ,
	})
}

// IncrementMetadataCounter -
func (p *Prometheus) IncrementMetadataCounter(network, typ, status string) {
	if p.service == nil {
		return
	}
	p.service.IncGaugeValue(MetricMetadataCounter, map[string]string{
		"network": network,
		"type":    typ,
		"status":  status,
	})
}

// IncrementErrorCounter -
func (p *Prometheus) IncrementErrorCounter(network string, err resolver.ResolvingError) {
	if p.service == nil {
		return
	}
	p.service.IncrementCounter(MetricsMetadataHttpErrors, map[string]string{
		"network": network,
		"type":    string(err.Type),
		"code":    strconv.FormatInt(int64(err.Code), 10),
	})
}

// AddHistogramResponseTime -
func (p *Prometheus) AddHistogramResponseTime(network string, data resolver.Resolved) {
	if p.service == nil {
		return
	}
	p.service.AddHistogramValue(MetricsMetadataIPFSResponseTime, map[string]string{
		"network": network,
		"node":    data.Node,
	}, float64(len(data.Data))/float64(data.ResponseTime))
}

// IncrementMimeCounter -
func (p *Prometheus) IncrementMimeCounter(network, mime string) {
	if p.service == nil {
		return
	}
	p.service.IncrementCounter("metadata_mime_type", map[string]string{
		"network": network,
		"mime":    mime,
	})
}

// SetMetadataNew -
func (p *Prometheus) SetMetadataNew(network, typ string, value float64) {
	if p.service == nil {
		return
	}
	p.service.SetGaugeValue(MetricMetadataNew, map[string]string{
		"network": network,
		"type":    typ,
	}, value)
}
