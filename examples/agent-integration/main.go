// Example: Using the Skillbox Go SDK in an AI agent.
//
// This demonstrates the primary integration pattern:
// create a client, run a skill, and handle the results.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	skillbox "github.com/devs-group/skillbox/sdks/go"
)

func main() {
	// Create a client. API key falls back to SKILLBOX_API_KEY env var.
	client := skillbox.New(
		"http://localhost:8080",
		os.Getenv("SKILLBOX_API_KEY"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Check server health
	if err := client.Health(ctx); err != nil {
		log.Fatalf("Server not reachable: %v", err)
	}
	fmt.Println("Server is healthy.")

	// List available skills
	skills, err := client.ListSkills(ctx)
	if err != nil {
		log.Fatalf("Failed to list skills: %v", err)
	}
	fmt.Printf("Available skills: %d\n", len(skills))
	for _, s := range skills {
		fmt.Printf("  - %s (v%s): %s\n", s.Name, s.Version, s.Description)
	}

	// Run the data-analysis skill
	result, err := client.Run(ctx, skillbox.RunRequest{
		Skill:   "data-analysis",
		Version: "latest",
		Input: json.RawMessage(`{
			"data": [
				{"name": "Alice", "age": 30, "score": 95},
				{"name": "Bob", "age": 25, "score": 87},
				{"name": "Charlie", "age": 35, "score": 92},
				{"name": "Diana", "age": 28, "score": 78},
				{"name": "Eve", "age": 32, "score": 99}
			]
		}`),
	})
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("\nExecution ID: %s\n", result.ExecutionID)
	fmt.Printf("Status:       %s\n", result.Status)
	fmt.Printf("Duration:     %dms\n", result.DurationMs)

	// Pretty-print the output
	if result.Output != nil {
		var output map[string]interface{}
		_ = json.Unmarshal(result.Output, &output)
		pretty, _ := json.MarshalIndent(output, "", "  ")
		fmt.Printf("Output:\n%s\n", pretty)
	}

	// Download file artifacts if present
	if result.HasFiles() {
		fmt.Printf("\nFiles: %v\n", result.FilesList)
		fmt.Printf("Download URL: %s\n", result.FilesURL)

		if err := client.DownloadFiles(ctx, result, "./output"); err != nil {
			log.Fatalf("Failed to download files: %v", err)
		}
		fmt.Println("Files downloaded to ./output/")
	}
}
