package main

import (
	"bytes"
	"log"
	"net/url"
	"strings"

	server "github.com/atharvamhaske/tcpie/internals"
	"github.com/atharvamhaske/tcpie/internals/config"
	"github.com/atharvamhaske/tcpie/internals/metrics"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

func main() {
	//load all configs using koanf
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(bytes.TrimSpace(config.ConfigFile)), yaml.Parser()); err != nil {
		log.Fatalf("error while loading config: %v", err)
	}

	var serverCfg config.ServerConfig
	if err := k.Unmarshal("server", &serverCfg); err != nil {
		log.Fatalf("error unmarshaling server config: %v", err)
	}

	var promCfg config.PromethuesConfig
	if err := k.Unmarshal("prometheus", &promCfg); err != nil {
		log.Fatalf("error unmarshaling prometheus config: %v", err)
	}

	serverURL := serverCfg.URL
	if parsedURL, err := url.Parse(serverCfg.URL); err == nil {
		if parsedURL.Host != "" {
			serverURL = parsedURL.Host
		} else if parsedURL.Scheme != "" {

			//if URL is like "http://localhost", extract just "localhost"
			serverURL = strings.TrimPrefix(strings.TrimPrefix(serverCfg.URL, "http://"), "https://")
		}
	}

	log.Printf("starting the server on %s:%d", serverURL, serverCfg.Port)

	// Get metrics endpoint and port from Prometheus config
	var metricsEndpoint string
	metricsPort := promCfg.MetricsPort

	if len(promCfg.ScrapeConfigs) > 0 {
		scrapeCfg := promCfg.ScrapeConfigs[0]
		metricsEndpoint = scrapeCfg.MetricsPath
	} else {
		metricsEndpoint = "/metrics"
	}

	exporter := metrics.NewExportMetrics(metricsPort, metricsEndpoint)
	opts := server.ServerOpts{
		MaxThreads: serverCfg.Workers,
		QueueSize:  serverCfg.QueueSize,
		Rate:       int64(serverCfg.TokenRate),
		Tokens:     int64(serverCfg.TokenLimit),
	}

	//create server object
	serverObject := &server.Server{
		Port:    serverCfg.Port,
		URL:     serverURL,
		Opts:    opts,
		Metrics: exporter.Metrics,
	}

	go exporter.ExportMetrics()
	log.Println("server and metrics exporter starting...")

	//start the TCP server (which blocks)
	serverObject.FireUpTheServer()
}
