package config

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var reopenTestPattern = regexp.MustCompile(`^[1-9]\d*:[1-9]\d*$`)

// Config holds every setting read from the environment.
type Config struct {
	AuthorID        int64
	ReopenTestMR    string // "project_id:iid"; enables reopen events for that MR only
	SigningToken    string // GitLab 19.1+ signing token (whsec_...)
	LegacySecret    string // GitLab secret token, older instances
	TelegramToken   string
	TelegramChatID  string
	TelegramAPIBase string
	ThreadID        int64
	SlackWebhookURL string
}

// Load reads settings through getenv so tests can supply their own environment.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		SigningToken:    getenv("GITLAB_WEBHOOK_SIGNING_TOKEN"),
		LegacySecret:    getenv("GITLAB_WEBHOOK_SECRET"),
		TelegramToken:   getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:  getenv("TELEGRAM_CHAT_ID"),
		TelegramAPIBase: getenv("TELEGRAM_API_BASE"),
		SlackWebhookURL: getenv("SLACK_WEBHOOK_URL"),
		ReopenTestMR:    getenv("REOPEN_TEST_MR"),
	}
	switch {
	case getenv("RENOVATE_AUTHOR_ID") == "":
		return c, errors.New("Missing environment settings: RENOVATE_AUTHOR_ID")
	case c.SigningToken == "" && c.LegacySecret == "":
		return c, errors.New("Missing environment settings: GitLab webhook credential")
	case c.SigningToken != "" && c.LegacySecret != "":
		return c, errors.New("Configure only one GitLab webhook credential")
	case (c.TelegramToken == "") != (c.TelegramChatID == ""):
		return c, errors.New("Set both TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID, or neither")
	case c.TelegramToken == "" && c.SlackWebhookURL == "":
		return c, errors.New("Configure Telegram and/or Slack")
	case c.SlackWebhookURL != "" && !strings.HasPrefix(c.SlackWebhookURL, "https://"):
		return c, errors.New("SLACK_WEBHOOK_URL must use HTTPS")
	}
	var err error
	if c.AuthorID, err = strconv.ParseInt(getenv("RENOVATE_AUTHOR_ID"), 10, 64); err != nil || c.AuthorID <= 0 {
		return c, errors.New("RENOVATE_AUTHOR_ID must be a positive integer")
	}
	if c.ReopenTestMR != "" && !reopenTestPattern.MatchString(c.ReopenTestMR) {
		return c, errors.New("REOPEN_TEST_MR must use project_id:iid, for example 316:588")
	}
	if v := getenv("TELEGRAM_MESSAGE_THREAD_ID"); v != "" {
		if c.ThreadID, err = strconv.ParseInt(v, 10, 64); err != nil {
			return c, errors.New("TELEGRAM_MESSAGE_THREAD_ID must be an integer")
		}
	}
	if c.TelegramAPIBase == "" {
		c.TelegramAPIBase = "https://api.telegram.org"
	}
	return c, nil
}
