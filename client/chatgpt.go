package client

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// --- Prompts and Messages
type Prompt interface {
	GetPurpose() string
	GetHistory() ([]string, []string)
	GetQuestion() string
	GetReferences() []io.Reader
}

type Transform interface {
	GetSource() io.Reader
	GetTarget() io.ReadWriter
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Reference struct {
	Contents io.Reader
}

func (r Reference) Value() ([]byte, error) {
	return io.ReadAll(r.Contents)
}

func MessageFromPrompt(prompt Prompt) []Message {
	messages := []Message{}
	messages = append(messages, Message{
		Role:    RoleSystem,
		Content: prompt.GetPurpose(),
	})
	givenInputs, idealOutputs := prompt.GetHistory()
	for i, givenInput := range givenInputs {
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: givenInput,
		})
		messages = append(messages, Message{
			Role:    RoleAssistant,
			Content: idealOutputs[i],
		})
	}
	messages = append(messages, Message{
		Role:    RoleUser,
		Content: prompt.GetQuestion(),
	})
	for i, reference := range prompt.GetReferences() {
		i++
		contents, err := io.ReadAll(reference)
		if err != nil {
			contents = []byte(fmt.Sprintf("Error reading reference: %v", err))
		}
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: fmt.Sprintf("Reference %d: %s", i, contents),
		})
	}

	return messages
}

// --- Dummy Client
type Dummy struct {
	FixedResponse string
	Failure       error
	T             Transform
	P             Prompt
}

func NewDummyClient(FixedResponse string, err error) *Dummy {
	return &Dummy{
		FixedResponse: FixedResponse,
		Failure:       err,
	}
}

func (d *Dummy) Completion(ctx context.Context, prompt Prompt) (io.Reader, error) {
	d.P = prompt
	return strings.NewReader(d.FixedResponse), d.Failure
}

func (d *Dummy) Transform(ctx context.Context, transform Transform) error {
	d.T = transform
	_, err := io.Copy(transform.GetTarget(), transform.GetSource())
	return err
}

// --- ChatGPT Client

type ChatGPT struct {
	Token string
	Model string
}

func NewChatGPT(token string) *ChatGPT {
	return &ChatGPT{
		Token: token,
		Model: GPT4,
	}
}

func (c *ChatGPT) Completion(ctx context.Context, prompt Prompt) (io.Reader, error) {
	return c.standardCompletion(ctx, prompt)
}

func (c *ChatGPT) audioCompletion(ctx context.Context, prompt Prompt) error {
	_, err := GenerateSpeech(c.Token, prompt.GetQuestion())
	if err != nil {
		return err
	}
	return err
}

func (c *ChatGPT) imageCompletion(ctx context.Context, prompt Prompt) ([]byte, error) {
	return GenerateImage(c.Token, prompt.GetQuestion())
}

func (c *ChatGPT) Transform(ctx context.Context, transform Transform) error {
	return nil
}
