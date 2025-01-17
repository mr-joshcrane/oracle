package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

type Prompt interface {
	GetPurpose() string
	GetHistory() ([]string, []string)
	GetQuestion() string
	GetReferences() [][]byte
	GetResponseFormat() []string
}

type Messages []Message

type Message interface {
	GetFormat() string
}

type TextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (m TextMessage) GetFormat() string {
	return ""
}

func MessageFromPrompt(prompt Prompt) Messages {
	messages := []Message{}
	messages = append(messages, TextMessage{
		Role:    RoleSystem,
		Content: prompt.GetPurpose(),
	})
	givenInputs, idealOutputs := prompt.GetHistory()
	for i, givenInput := range givenInputs {
		messages = append(messages, TextMessage{
			Role:    RoleUser,
			Content: givenInput,
		})
		messages = append(messages, TextMessage{
			Role:    RoleAssistant,
			Content: idealOutputs[i],
		})
	}
	messages = append(messages, TextMessage{
		Role:    RoleUser,
		Content: prompt.GetQuestion(),
	})
	refs := prompt.GetReferences()
	for i, ref := range refs {
		i++
		if isPNG(ref) {
			uri := ConvertPNGToDataURI(ref)
			messages = append(messages, VisionMessage{
				Role: RoleUser,
				Content: []VisionImageURL{
					{
						Type: "image_url",
						ImageURL: struct {
							URL string `json:"url"`
						}{
							URL: uri,
						},
					},
				}})
			continue
		}
		messages = append(messages, TextMessage{
			Role:    RoleUser,
			Content: fmt.Sprintf("Reference %d: %s", i, ref),
		})
	}
	return messages
}

func Do(ctx context.Context, token string, model ModelConfig, prompt Prompt) (io.Reader, error) {
	format := prompt.GetResponseFormat()
	strategy := textCompletion
	refs := prompt.GetReferences()
	for _, ref := range refs {
		if isPNG(ref) {
			strategy = visionCompletion
		}
	}
	messages := MessageFromPrompt(prompt)
	return strategy(ctx, token, model, messages, format...)
}

func addDefaultHeaders(token string, r *http.Request) *http.Request {
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", "Bearer "+token)
	return r
}
