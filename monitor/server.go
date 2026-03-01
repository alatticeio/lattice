package monitor

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartServer(port string) error {
	http.Handle("/metrics", promhttp.Handler())

	return http.ListenAndServe(":"+port, nil)
}
