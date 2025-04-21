package main

import (
	"github.com/ls1intum/hades/benchmarkingSuite/persister"
	"log/slog"
	"os"
)

// Persister handles to store the job results in the database
var p persister.Persister

func main() {
	// Set the log level to debug if the DEBUG environment variable is set to true
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		slog.Warn("DEBUG MODE ENABLED")
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	slog.Debug("Creating DB persister")
	p = persister.NewDBPersister()

	r := startRouter()

	// TODO: Add this to a configuration file
	r.Run(":8080")
}
