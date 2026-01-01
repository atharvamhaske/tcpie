package config

import (
	_ "embed"
)

//go:embed config.yaml
var ConfigFile []byte

type ServerConfig struct {
	URL        string `koanf:"url"`
	Name       string `koanf:"name"`
	Port       int    `koanf:"port"`
	Workers    int    `koanf:"workers"`
	QueueSize  int    `koanf:"queue_size"`
	TokenRate  int    `koanf:"token_rate"`
	TokenLimit int    `koanf:"token_limit"`
}

type PromethuesConfig struct {
	MetricsPort int64 `koanf:"metrics_port"`
	Global      struct {
		ScrapeInterval   string `koanf:"scrape_interval"`
		EvaluateInterval string `koanf:"evaluate_interval"`
	} `koanf:"global"`

	ScrapeConfigs []struct {
		JobName       string `koanf:"job_name"`
		MetricsPath   string `koanf:"metrics_path"`
		StaticConfigs []struct {
			Targets []string `koanf:"targets"`
		} `koanf:"static_configs"`
	} `koanf:"scrape_configs"`
}

type Configs struct {
	Server     ServerConfig     `koanf:"server"`
	Promethues PromethuesConfig `koanf:"promethues"`
} //exports all above structs config cleanly to use
