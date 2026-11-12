package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	botToken   string
}

func NewClient(botToken string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		botToken:   botToken,
	}
}

type messagePayload struct {
	Content string `json:"content"`
}

func (c *Client) SendChannelMessage(channelID int64, content string) error {
	if c.botToken == "" {
		return fmt.Errorf("discord bot token is empty")
	}

	chunks := splitMessage(content, 1900)
	for _, chunk := range chunks {
		if err := c.postMessage(channelID, chunk); err != nil {
			return err
		}
		if len(chunks) > 1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func (c *Client) postMessage(channelID int64, content string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%d/messages", channelID)
	data, err := json.Marshal(messagePayload{Content: content})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cron-stb")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func splitMessage(message string, limit int) []string {
	if len(message) <= limit {
		return []string{message}
	}

	var chunks []string
	var current string
	for _, line := range strings.Split(message, "\n") {
		candidate := line
		if current != "" {
			candidate = current + "\n" + line
		}

		if len(candidate) > limit {
			if current != "" {
				chunks = append(chunks, current)
			}
			current = line
		} else {
			current = candidate
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}
