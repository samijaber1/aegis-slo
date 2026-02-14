package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/samijaber1/aegis-slo/internal/adapter/prometheus"
	"github.com/samijaber1/aegis-slo/internal/adapter/synthetic"
	"github.com/samijaber1/aegis-slo/internal/api"
	"github.com/samijaber1/aegis-slo/internal/config"
	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/scheduler"
)

func main() {
	// Parse flags
	cfg := parseFlags()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting AegisSLO server...")
	log.Printf("Config: port=%d, slo-dir=%s, adapter=%s", cfg.Port, cfg.SLODirectory, cfg.AdapterType)

	// Create metrics adapter
	var metricsAdapter eval.MetricsAdapter
	switch cfg.AdapterType {
	case "prometheus":
		promConfig := prometheus.DefaultConfig(cfg.PrometheusURL)
		metricsAdapter = prometheus.NewAdapter(promConfig)
		log.Printf("Using Prometheus adapter: %s", cfg.PrometheusURL)

	case "synthetic":
		metricsAdapter = synthetic.NewAdapter()
		// Load fixtures if directory specified
		if cfg.SyntheticFixDir != "" {
			// Synthetic fixtures would be loaded here
			log.Printf("Using synthetic adapter with fixtures from: %s", cfg.SyntheticFixDir)
		} else {
			log.Printf("Using synthetic adapter (no fixtures directory specified)")
		}

	default:
		log.Fatalf("Unknown adapter type: %s", cfg.AdapterType)
	}

	// Create evaluator and policy engine
	evaluator := eval.NewEvaluator(metricsAdapter)
	policyEngine := policy.NewEngine()

	// Create scheduler
	sched := scheduler.NewScheduler(evaluator, policyEngine, cfg.SLODirectory)

	// Load SLOs
	if err := sched.LoadSLOs(); err != nil {
		log.Fatalf("Failed to load SLOs: %v", err)
	}

	// Start scheduler
	if err := sched.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Create and start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	apiServer := api.NewServer(sched, addr)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- apiServer.Start()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)

	case sig := <-shutdown:
		log.Printf("Received signal: %v", sig)

		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
		defer cancel()

		log.Println("Shutting down server...")
		if err := apiServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}

		log.Println("Stopping scheduler...")
		sched.Stop()

		log.Println("Shutdown complete")
	}
}

func parseFlags() config.Config {
	cfg := config.DefaultConfig()

	flag.IntVar(&cfg.Port, "port", cfg.Port, "HTTP server port")
	flag.StringVar(&cfg.Host, "host", cfg.Host, "HTTP server host")
	flag.StringVar(&cfg.SLODirectory, "slo-dir", cfg.SLODirectory, "Directory containing SLO YAML files")
	flag.StringVar(&cfg.AdapterType, "adapter", cfg.AdapterType, "Metrics adapter type (prometheus|synthetic)")
	flag.StringVar(&cfg.PrometheusURL, "prometheus-url", cfg.PrometheusURL, "Prometheus server URL (required for prometheus adapter)")
	flag.StringVar(&cfg.SyntheticFixDir, "synthetic-fixtures", cfg.SyntheticFixDir, "Directory containing synthetic metric fixtures")

	flag.Parse()

	return cfg
}
