package webmailtester

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	webmailTTFB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "webmail_ttfb_seconds",
			Help:      "Time to first byte for webmail",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	webmailLoginTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "webmail_login_time_seconds",
			Help:      "Time to authenticate to webmail",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	webmailFirstPageTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "webmail_first_page_time_seconds",
			Help:      "Time to load first page",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	webmailMessageLoadTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "webmail_message_load_time_seconds",
			Help:      "Time to load message",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	webmailErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "webmail_errors",
			Help:      "Number of webmail errors by type",
			Namespace: "mailmetrix",
		},
		[]string{"server", "type"},
	)
	webmailFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "webmail_failures_total",
			Help:      "Total number of webmail operation failures",
			Namespace: "mailmetrix",
		},
		[]string{"server", "operation"},
	)
)

func init() {
	metrics := []prometheus.Collector{
		webmailTTFB,
		webmailLoginTime,
		webmailFirstPageTime,
		webmailMessageLoadTime,
		webmailErrors,
	}

	for _, m := range metrics {
		prometheus.MustRegister(m)
	}
}
