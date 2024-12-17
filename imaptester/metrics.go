package imaptester

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	timeToBanner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "imap_time_to_banner_seconds",
			Help:      "Time to receive IMAP banner",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	timeToAuth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "imap_time_to_auth_seconds",
			Help:      "Time to authenticate to IMAP server",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	timeToFetch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "imap_time_to_fetch_seconds",
			Help:      "Time to fetch messages from IMAP server",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	timeToAppend = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "imap_time_to_append_seconds",
			Help:      "Time to append message to IMAP server",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
	timeToExpunge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "imap_time_to_expunge_seconds",
			Help:      "Time to expunge messages from IMAP server",
			Namespace: "mailmetrix",
		},
		[]string{"server"},
	)
)

func init() {
	metrics := []prometheus.Collector{
		timeToBanner,
		timeToAuth,
		timeToFetch,
		timeToAppend,
		timeToExpunge,
	}

	for _, metric := range metrics {
		if err := prometheus.Register(metric); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				prometheus.Unregister(are.ExistingCollector)
				prometheus.MustRegister(metric)
			} else {
				log.Printf("Error registering metric: %v", err)
			}
		}
	}
}
