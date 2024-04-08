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
var ChatURL = "/api/chat"

// BasicResponse
type BasicResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Done               bool   `json:"done"`
	TotalDuration      int    `json:"total_duration,omitempty"`
	LoadDuration       int    `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int    `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int    `json:"eval_duration,omitempty"`
}

// Message represents the structure of each JSON object in the stream
type Message struct {
	Response string `json:"response"`
	BasicResponse
}

// /chat API data defines
type ChatResponse struct {
	Message ChatMessage `json:"message" mapstructure:"message"`
	BasicResponse
}

type ChatMessage struct {
	Role    string `json:"role" mapstructure:"role"` // the role of the message, either system, user or assistant
	Content string `json:"content" mapstructure:"content"`
}

type ChatParameters struct {
	Format  string         `json:"format,omitempty" mapstructure:"format"`
	Stream  bool           `json:"stream,omitempty" mapstructure:"stream"`
	Options map[string]any `json:"options,omitempty" mapstructure:"options"`
}

// /chat API
type ChatRequest struct {
	Model    string        `json:"model" mapstructure:"model"`
	Messages []ChatMessage `json:"messages" mapstructure:"messages"`
}

// RequestChat sends a request to the /chat API
func RequestChat(model string, messages []ChatMessage, stream chan<- string) (string, error) {

	apiURL := fmt.Sprintf("http://%s:%s%s", ollamaHost, ollamaPort, ChatURL)
	body := ChatRequest{
		Model:    model,
		Messages: messages,
	}
	jsonByte, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}
	buffer := bytes.NewBuffer(jsonByte)
	resp, err := http.Post(apiURL, "application/json", buffer)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response status: %d", resp.StatusCode)
	}
	var result string
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		// fmt.Printf("%s\n", line)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading from stream: %v", err)
		}
		var response ChatResponse
		if err := json.NewDecoder(bytes.NewReader(line)).Decode(&response); err != nil {
			fmt.Printf("err %v\n", err)
			return "", fmt.Errorf("error decoding JSON: %v", err)
		}

		stream <- response.Message.Content
		result += response.Message.Content

		if response.Done {
			break
		}
	}

	return result, nil
}

// Translater
func Translater(msg string) {
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "Translate the following text to English",
		},
		{
			Role:    "user",
			Content: msg,
		},
	}
	stream := make(chan string)

	go func() {
		defer close(stream)
		if _, err := RequestChat(model, messages, stream); err != nil {
			log.Fatalf("Failed to request chat: %v", err)

		}
	}()

	for {
		select {
		case text, ok := <-stream:
			if !ok { // channel is closed
				return
			}
			fmt.Printf("%s", text)

		}
	}

}

var ollamaHost string
var ollamaPort string
var model string

func main() {
	// flags set ollama host and port

	var showSummary bool
	flag.StringVar(&ollamaHost, "h", "localhost", "ollama host")
	flag.StringVar(&ollamaPort, "p", "11434", "ollama port")
	// flag show summary
	flag.BoolVar(&showSummary, "s", false, "show summary")
	// model

	flag.StringVar(&model, "model", "llama2", "model")
	// response json
	var responseJSON bool
	flag.BoolVar(&responseJSON, "j", false, "response json")

	// translate mode
	var translate bool
	flag.BoolVar(&translate, "t", false, "translate mode")

	flag.Parse()

	if translate {
		Translater(strings.Join(flag.Args(), " "))
		return
	}
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
