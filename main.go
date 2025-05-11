package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/veverkap/gmail-to-sqlite/auth"
	"github.com/veverkap/gmail-to-sqlite/db"
	"github.com/veverkap/gmail-to-sqlite/sync"
)

// prepareDataDir creates the data directory if it doesn't exist.
func prepareDataDir(dataDir string) error {
	return os.MkdirAll(dataDir, 0755)
}

func main() {
	var (
		dataDir   string
		fullSync  bool
		messageID string
	)

	// Define commands
	syncCommand := flag.NewFlagSet("sync", flag.ExitOnError)
	syncCommand.StringVar(&dataDir, "data-dir", "", "The path where the data should be stored")
	syncCommand.BoolVar(&fullSync, "full-sync", false, "Force a full sync of all messages")

	syncMessageCommand := flag.NewFlagSet("sync-message", flag.ExitOnError)
	syncMessageCommand.StringVar(&dataDir, "data-dir", "", "The path where the data should be stored")
	syncMessageCommand.StringVar(&messageID, "message-id", "", "The ID of the message to sync")

	// Ensure a command is provided
	if len(os.Args) < 2 {
		fmt.Println("expected 'sync' or 'sync-message' subcommands")
		os.Exit(1)
	}

	// Parse the appropriate command
	switch os.Args[1] {
	case "sync":
		syncCommand.Parse(os.Args[2:])
	case "sync-message":
		syncMessageCommand.Parse(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("expected 'sync' or 'sync-message' subcommands")
		os.Exit(1)
	}

	// Check if data directory is provided
	if dataDir == "" {
		fmt.Println("Please provide a --data-dir")
		os.Exit(1)
	}

	// Ensure data directory exists
	if err := prepareDataDir(dataDir); err != nil {
		fmt.Printf("Failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	// Get OAuth2 token
	token, err := auth.GetCredentials(dataDir)
	if err != nil {
		fmt.Printf("Failed to get credentials: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.Init(dataDir, false)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Execute the requested command
	switch {
	case syncCommand.Parsed():
		count, err := sync.AllMessages(token, database, fullSync)
		if err != nil {
			fmt.Printf("Failed to sync messages: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Synced %d messages\n", count)

	case syncMessageCommand.Parsed():
		if messageID == "" {
			fmt.Println("Please provide a --message-id")
			os.Exit(1)
		}
		if err := sync.SingleMessage(token, database, messageID); err != nil {
			fmt.Printf("Failed to sync message: %v\n", err)
			os.Exit(1)
		}
	}
}