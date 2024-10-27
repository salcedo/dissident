package dissident

import (
	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: plugin.Namespace,
	Subsystem: "dissident",
	Name:      "request_count_total",
	Help:      "Counter of requests seen by dissident plugin.",
}, []string{"server"})

var allowedQueries = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: plugin.Namespace,
	Subsystem: "dissident",
	Name:      "allowed_queries_total",
	Help:      "Counter of allowed queries.",
}, []string{"server"})

var blockedQueries = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: plugin.Namespace,
	Subsystem: "dissident",
	Name:      "blocked_queries_total",
	Help:      "Counter of blocked queries.",
}, []string{"server"})

var once sync.Once
