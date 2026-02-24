package stockyard

import (
	"context"
	"github.com/sashabaranov/go-openai"
)

// StockyardActivity wraps LLM calls through Stockyard proxy
func StockyardActivity(ctx context.Context, prompt string) (string, error) {
	client := openai.NewClientWithConfig(openai.ClientConfig{
		BaseURL: "http://localhost:4000/v1",
	})
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
