package anthropic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
)

type Role string

var (
	User Role = "USER"
	Bot  Role = "ASSISTANT"
)

type Prompt interface {
	GetPurpose() string
	GetHistory() ([]string, []string)
	GetQuestion() string
	GetReferences() [][]byte
}

type ChatMessage struct {
	Role  Role        `json:"role"`
	Parts MessagePart `json:"parts"`
}

type MessagePart struct {
	Text string `json:"text,omitempty"`
}

type Anthropic struct {
	Token string
	Model ModelConfig
}

func NewAnthropic(token string) *Anthropic {
	return &Anthropic{
		Token: token,
		Model: Models["ClaudeSonnet"],
	}
}

func Authenticate() (token string, err error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	return key, nil
}

func Completion(ctx context.Context, token string, model ModelConfig, prompt Prompt) (io.Reader, error) {
	req, err := createCompletionRequest(ctx, token, model, prompt)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseAnthropicResponse(resp)
}

func createCompletionRequest(ctx context.Context, token string, model ModelConfig, prompt Prompt) (*http.Request, error) {

	messages := createAnthropicMessages(prompt)
	data, _ := json.Marshal(messages)
	os.WriteFile("messages.json", data, 0644)
	requestBody := map[string]any{
		"model":      model.Name,
		"max_tokens": 1024,
		"messages":   messages,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.anthropic.com/v1/messages",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", token)
	return req, nil
}

func isValidImage(ref []byte) bool {
	reader := bytes.NewReader(ref)
	_, _, err := image.Decode(reader)
	return err == nil
}

type Message struct {
	Role    Role `json:"role"`
	Content any  `json:"content"`
}

type ImagePayload struct {
	Type   string `json:"type"`
	Source struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"source"`
}

func createImageContent(imageData []byte) ImagePayload {
	return ImagePayload{
		Type: "image",
		Source: struct {
			Type      string `json:"type"`
			MediaType string `json:"media_type"`
			Data      string `json:"data"`
		}{
			Type:      "base64",
			MediaType: "image/jpeg",
			Data:      base64.StdEncoding.EncodeToString(imageData),
		},
	}
}
func createAnthropicMessages(prompt Prompt) []Message {
	messages := []Message{}
	userHistory, assistantHistory := prompt.GetHistory()
	for i := range userHistory {
		messages = append(messages, Message{Role: "user", Content: userHistory[i]})
		messages = append(messages, Message{Role: "assistant", Content: assistantHistory[i]})
	}
	messages = append(messages, Message{Role: "user", Content: prompt.GetQuestion()})
	for _, ref := range prompt.GetReferences() {
		isImage := isValidImage(ref)
		if !isImage {
			messages = append(messages, Message{Role: "user", Content: string(ref)})
			continue
		}
		imagePayload := createImageContent(ref)
		messages = append(messages, Message{Role: "user", Content: []ImagePayload{imagePayload}})
	}
	return messages
}

func parseAnthropicResponse(resp *http.Response) (io.Reader, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d; %s", resp.StatusCode, resp.Status)
	}

	var responseBody struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}
	var completion string
	for _, message := range responseBody.Content {
		completion += message.Text
	}
	return strings.NewReader(completion), nil
}