package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanywst/zanzibar/src/api"
	"github.com/kanywst/zanzibar/src/policy"
	"github.com/kanywst/zanzibar/src/schema"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to listen on")
	initSample := flag.Bool("sample", true, "Initialize with sample data")
	flag.Parse()

	// Initialize schema
	log.Println("Initializing schema...")
	schemaStore := schema.LoadDefaultSchema()

	// Initialize policy store
	log.Println("Initializing policy store...")
	policyStore := policy.NewStore(schemaStore)

	// Initialize with sample data if requested
	if *initSample {
		log.Println("Initializing with sample data...")
		policyStore.InitializeWithSampleData()
	}

	// Create API server
	log.Println("Creating API server...")
	server := api.NewServer(policyStore, schemaStore)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting server on port %d...", *port)
	if err := server.Start(*port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
