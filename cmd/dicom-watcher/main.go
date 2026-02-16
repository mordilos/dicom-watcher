package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"ikh/dicom-watcher/internal/config"
	"ikh/dicom-watcher/internal/watcher"
)

func main() {
	// Read the configuration file
	log.Print("Reading config...")
	config, err := config.ReadConfig("/app/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the watcher
	log.Print("Initializing the watcher...")
	watcher, err := watcher.NewWatcher(config)
	if err != nil {
		log.Fatal(err)
	}

	// Start the watcher
	log.Print("Starting the watcher...")
	watcher.Start()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
