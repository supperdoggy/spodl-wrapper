package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	// Define the file name
	inputFile := "album_to_download.txt"

	// Open the file
	file, err := os.Open(inputFile)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text()) // Remove any surrounding whitespace

		// Skip empty lines
		if url != "" {
			// Print the URL being processed
			fmt.Printf("Downloading: %s\n", url)

			// Run the "spotdl --sync {url}" command
			cmd := exec.Command("spotdl", "--sync", url, "--cookie-file cookies.txt", "--bitrate disable")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Execute the command
			if err := cmd.Run(); err != nil {
				log.Printf("Failed to execute command for URL %s: %v", url, err)
			}

			// Sleep for 2 seconds before processing the next URL
			time.Sleep(2 * time.Second)
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %s", err)
	}
}
