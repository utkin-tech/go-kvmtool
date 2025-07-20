package main

import (
	"flag"
)

type Config struct {
	kernelFilename string
}

func BuildOptions(cfg *Config) {
	flag.StringVar(&cfg.kernelFilename, "k", "", "Kernel to boot in virtual machine")
	flag.StringVar(&cfg.kernelFilename, "kernel", "", "Kernel to boot in virtual machine")
}

func ParseConfig() *Config {
	cfg := &Config{}

	BuildOptions(cfg)

	flag.Parse()

	return cfg
}
