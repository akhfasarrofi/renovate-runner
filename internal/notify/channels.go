package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"renovate-notifier/internal/config"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

// Sender delivers one message to one chat channel.
type Sender struct {
	Name string
	Send func(message string) error
}

// Senders returns the channels enabled by the configuration.
func Senders(c config.Config) []Sender {
	var out []Sender
	if c.TelegramToken != "" {
		out = append(out, Sender{"telegram", func(m string) error {
			return sendTelegram(c.TelegramToken, c.TelegramChatID, m, c.TelegramAPIBase, c.ThreadID)
		}})
	}
	if c.SlackWebhookURL != "" {
		out = append(out, Sender{"slack", func(m string) error { return sendSlack(c.SlackWebhookURL, m) }})
	}
	return out
}

func sendTelegram(token, chatID, message, apiBase string, threadID int64) error {
	data := map[string]any{
		"chat_id":              chatID,
		"text":                 message,
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	if threadID != 0 {
		data["message_thread_id"] = threadID
	}
	out, err := postJSON(strings.TrimRight(apiBase, "/")+"/bot"+token+"/sendMessage", data, 1<<20)
	if err != nil {
		return err
	}
	var result struct{ OK bool }
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("Telegram response: %w", err)
	}
	if !result.OK {
		return errors.New("Telegram rejected the message")
	}
	return nil
}

// Slack interprets <...> as references. Escape data from MR titles.
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func sendSlack(webhookURL, message string) error {
	out, err := postJSON(webhookURL, map[string]string{"text": slackEscaper.Replace(message)}, 64)
	if err != nil {
		return err
	}
	if string(bytes.TrimSpace(out)) != "ok" {
		return errors.New("Slack rejected the message")
	}
	return nil
}

// postJSON posts data and returns at most limit bytes of a 2xx response body.
func postJSON(target string, data any, limit int64) ([]byte, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Post(target, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, cleanErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return out, cleanErr(err)
}

// cleanErr drops the request URL, which holds the Telegram token or Slack webhook secret.
func cleanErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s request failed: %w", ue.Op, ue.Err)
	}
	return err
}
