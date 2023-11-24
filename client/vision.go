package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
)

const (
	DALLE3 = "dall-e-3"
	GPT4V  = "gpt-4-vision-preview"
)

func ImageToDataURI(img image.Image) (string, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return "", err
	}
	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	dataURI := "data:image/png;base64," + base64Str
	return dataURI, nil
}

func URLToURI(url url.URL) (string, error) {
	visionMimeType := []string{
		"image/png",
		"image/jpeg",
		"image/jpg",
	}
	resp, err := http.DefaultClient.Head(url.String())
	if err != nil {
		return "", err
	}
	for _, mimeType := range visionMimeType {
		if resp.Header.Get("Content-Type") == mimeType {
			return url.String(), nil
		}
	}
	return "", fmt.Errorf("unsupported visual mime type: %s", resp.Header.Get("Content-Type"))
}

type ImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type ImageResponse struct {
	Created int `json:"created"`
	Data    []struct {
		Url string `json:"url"`
	} `json:"data"`
}

func CreateImageRequest(token string, prompt string) (*http.Request, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(ImageRequest{
		Model:  DALLE3,
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/images/generations", buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func GenerateImage(token, prompt string) (string, error) {
	req, err := CreateImageRequest(token, prompt)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println(resp.Status)
		return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var image ImageResponse
	err = json.Unmarshal(data, &image)
	if err != nil {
		return "", err
	}
	if len(image.Data) < 1 {
		return "", fmt.Errorf("no images returned")
	}
	return image.Data[0].Url, nil

}

type VisionImageURL struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

type VisionMessage struct {
	Role    string              `json:"role"`
	Content []map[string]string `json:"content"`
}

type VisionRequest struct {
	Model     string          `json:"model"`
	Messages  []VisionMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

func CreateVisionRequest(token string, message VisionMessage) (*http.Request, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(VisionRequest{
		Model:     GPT4V,
		Messages:  []VisionMessage{message},
		MaxTokens: 300,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

type VisionCompletionResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishDetails struct {
			Type string `json:"type"`
		} `json:"finish_details"`
		Index int `json:"index"`
	} `json:"choices"`
}

func CreateVisionMessage(prompt string, images ...string) VisionMessage {
	messages := VisionMessage{
		Role: RoleUser,
		Content: []map[string]string{
			{
				"type": "text",
				"text": prompt,
			},
		},
	}
	for _, imageSrc := range images {
		messages.Content = append(messages.Content, map[string]string{
			"type":      "image_url",
			"image_url": imageSrc,
		})
	}
	return messages
}

func (c *ChatGPT) visionCompletion(ctx context.Context, prompt string, images ...string) (string, error) {
	message := CreateVisionMessage(prompt, images...)
	req, err := CreateVisionRequest(c.Token, message)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", NewClientError(resp)
	}
	defer resp.Body.Close()
	var completion VisionCompletionResponse
	err = json.NewDecoder(resp.Body).Decode(&completion)
	if err != nil {
		return "", err
	}
	if len(completion.Choices) < 1 {
		return "", fmt.Errorf("no choices returned")
	}
	return completion.Choices[0].Message.Content, nil
}
