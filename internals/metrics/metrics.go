package metrics

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerMetrics struct for server metrics using prometheus
type ServerMetrics struct {
	Requests *prometheus.CounterVec
}

// used to export metrics captures to prometheus
type MetricsExport struct {
	Metrics  ServerMetrics //metrics that server supports
	Port     int64         //port in which exporter will run
	Endpoint string        //endpoint which promethues will call to get scrap metrics
}

func (s *ServerMetrics) CreateMetrics() {
	s.Requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "total_requests",
			Help: "Number of requests proccessed by a server",
		},
		[]string{"Processed"},
	)
}

func (e *MetricsExport) ExportMetrics() {
	r := mux.NewRouter()

	r.Path(e.Endpoint).Handler(promhttp.Handler())
	log.Printf("Starting metrics exporter on port: %d", e.Port)

	err := http.ListenAndServe(":"+ fmt.Sprintf("%d", e.Port), r)
	log.Fatal(err)
}

func NewServerMetrics() ServerMetrics {
	reqMetrics := ServerMetrics{}
	reqMetrics.CreateMetrics()
	prometheus.Register(reqMetrics.Requests)

	return reqMetrics
}

func NewExportMetrics (port int64, endpoint string) MetricsExport {
	metrics := NewServerMetrics()
	exporter := MetricsExport{Port: port}
	exporter.Metrics = metrics
	exporter.Endpoint = endpoint

	return exporter
}