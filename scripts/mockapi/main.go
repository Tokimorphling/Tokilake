package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

var addr string

func main() {
	flag.StringVar(&addr, "addr", ":18081", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Printf("Mock API server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	log.Printf("Received request: model=%s messages=%d stream=%v", req.Model, len(req.Messages), req.Stream)

	userMessage := ""
	if len(req.Messages) > 0 {
		userMessage = req.Messages[len(req.Messages)-1].Content
	}

	reply := fmt.Sprintf("test response from mock api: received '%s' for model %s", userMessage, req.Model)

	if req.Stream {
		handleStreamResponse(w, req.Model, reply)
	} else {
		handleNonStreamResponse(w, req.Model, reply)
	}
}

func handleNonStreamResponse(w http.ResponseWriter, model, reply string) {
	resp := map[string]any{
		"id":      "chatcmpl-test-123",
		"object":  "chat.completion",
		"created": 1234567890,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": reply,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStreamResponse(w http.ResponseWriter, model, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	chunks := strings.Split(reply, " ")
	for i, chunk := range chunks {
		isLast := i == len(chunks)-1
		data := map[string]any{
			"id":      "chatcmpl-test-123",
			"object":  "chat.completion.chunk",
			"created": 1234567890,
			"model":   model,
			"choices": []map[string]any{
				{
					"index": i,
					"delta": map[string]string{
						"content": chunk + " ",
					},
					"finish_reason": nil,
				},
			},
		}
		if isLast {
			data["choices"] = []map[string]any{
				{
					"index": i,
					"delta": map[string]string{},
					"finish_reason": "stop",
				},
			}
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}
