package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

var GenerateURL = "/api/generate"

// Message represents the structure of each JSON object in the stream
type Message struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"` // Changed from a nested Message struct to a direct string field
	Done               bool   `json:"done"`
	TotalDuration      int    `json:"total_duration,omitempty"`
	LoadDuration       int    `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int    `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int    `json:"eval_duration,omitempty"`
	// You might also want to include the other fields like 'context', 'total_duration', etc., based on your needs
}

func main() {
	// flags set ollama host and port
	var ollamaHost string
	var ollamaPort string
	var showSummary bool
	flag.StringVar(&ollamaHost, "h", "localhost", "ollama host")
	flag.StringVar(&ollamaPort, "p", "11434", "ollama port")
	// flag show summary
	flag.BoolVar(&showSummary, "s", false, "show summary")
	// model
	var model string
	flag.StringVar(&model, "model", "llama2", "model")
	// response json
	var responseJSON bool
	flag.BoolVar(&responseJSON, "j", false, "response json")
	flag.Parse()

	// URL of the HTTP stream
	url := fmt.Sprintf("http://%s:%s%s", ollamaHost, ollamaPort, GenerateURL)
	prompt := strings.Join(flag.Args(), " ")
	body := map[string]any{
		"prompt": prompt,
		"model":  model,
	}
	if responseJSON {
		body["format"] = "json"
		body["stream"] = false
	}
	jsonByte, err := json.Marshal(body)
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	buffer := bytes.NewBuffer(jsonByte)
	// Make a request to the URL
	resp, err := http.Post(url, "application/json", buffer)
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received non-200 response status: %d", resp.StatusCode)
	}

	if responseJSON {
		// read all resp.Body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading from stream: %v", err)
		}
		fmt.Printf("%s", body)
		return
	}

	// Read the stream line by line
	reader := bufio.NewReader(resp.Body)
	fmt.Printf(">>>\n")
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break // End of the stream
		}
		if err != nil {
			log.Fatalf("Error reading from stream: %v", err)
		}

		var msg Message
		if err := json.NewDecoder(bytes.NewReader(line)).Decode(&msg); err != nil {
			log.Printf("Error decoding JSON: %v", err)
			continue
		}

		// Display the response directly
		fmt.Printf("%s", msg.Response)

		if msg.Done {
			// if show summary flag is set
			if showSummary {
				// print response
				fmt.Printf("\n\nline: %s\n", line)
				// Print all duration convert nanoseconds to seconds and round to 2 decimal places
				fmt.Printf("Total Duration: %.2f seconds\n", float64(msg.TotalDuration)/1e9)
				fmt.Printf("Load Duration: %.2f seconds\n", float64(msg.LoadDuration)/1e9)
				fmt.Printf("Prompt Eval Count: %d\n", msg.PromptEvalCount)
				fmt.Printf("Prompt Eval Duration: %.2f seconds\n", float64(msg.PromptEvalDuration)/1e9)
				fmt.Printf("Eval Count: %d\n", msg.EvalCount)
				fmt.Printf("Eval Duration: %.2f seconds\n", float64(msg.EvalDuration)/1e9)

			}

			break
		}
	}
}
