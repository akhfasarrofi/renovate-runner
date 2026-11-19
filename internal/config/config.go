package config

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var reopenTestPattern = regexp.MustCompile(`^[^:\s]+:[1-9]\d*$`)

// Config holds every setting read from the environment.
type Config struct {
	Platform        string // gitlab (default), github or bitbucket
	Author          string // bot identity: GitLab user ID, GitHub login, Bitbucket account_id/uuid
	ReopenTestMR    string // "repo_id:iid"; enables reopen events for that MR only
	Secret          string // webhook secret, all platforms
	SigningToken    string // GitLab 19.1+ signing token (whsec_...)
	Lang            string // message language: en (default) or id
	TelegramToken   string
	TelegramChatID  string
	ThreadID        int64
	SlackWebhookURL string
}

// first returns the first non-empty value of the given variables.
func first(getenv func(string) string, names ...string) string {
	for _, n := range names {
		if v := getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// Load reads settings through getenv so tests can supply their own environment.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Platform:        first(getenv, "RENOVATE_PLATFORM"),
		Author:          first(getenv, "RENOVATE_AUTHOR"),
		Secret:          first(getenv, "WEBHOOK_SECRET"),
		SigningToken:    getenv("GITLAB_WEBHOOK_SIGNING_TOKEN"),
		Lang:            first(getenv, "NOTIFY_LANG"),
		TelegramToken:   getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:  getenv("TELEGRAM_CHAT_ID"),
		SlackWebhookURL: getenv("SLACK_WEBHOOK_URL"),
		ReopenTestMR:    getenv("REOPEN_TEST_MR"),
	}
	if c.Platform == "" {
		c.Platform = "gitlab"
	}
	if c.Lang == "" {
		c.Lang = "en"
	}
	switch {
	case c.Platform != "gitlab" && c.Platform != "github" && c.Platform != "bitbucket":
		return c, errors.New("RENOVATE_PLATFORM must be gitlab, github or bitbucket")
	case c.Lang != "en" && c.Lang != "id":
		return c, errors.New("NOTIFY_LANG must be en or id")
	case c.Author == "":
		return c, errors.New("Missing environment settings: RENOVATE_AUTHOR")
	case c.Secret == "" && c.SigningToken == "":
		return c, errors.New("Missing environment settings: WEBHOOK_SECRET")
	case c.Secret != "" && c.SigningToken != "":
		return c, errors.New("Configure only one webhook credential")
	case c.SigningToken != "" && c.Platform != "gitlab":
		return c, errors.New("GITLAB_WEBHOOK_SIGNING_TOKEN only works with RENOVATE_PLATFORM=gitlab")
	case (c.TelegramToken == "") != (c.TelegramChatID == ""):
		return c, errors.New("Set both TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID, or neither")
	case c.TelegramToken == "" && c.SlackWebhookURL == "":
		return c, errors.New("Configure Telegram and/or Slack")
	case c.SlackWebhookURL != "" && !strings.HasPrefix(c.SlackWebhookURL, "https://"):
		return c, errors.New("SLACK_WEBHOOK_URL must use HTTPS")
	}
	if c.Platform == "gitlab" {
		if id, err := strconv.ParseInt(c.Author, 10, 64); err != nil || id <= 0 {
			return c, errors.New("RENOVATE_AUTHOR must be the GitLab user ID (a positive integer)")
		}
	}
	if c.ReopenTestMR != "" && !reopenTestPattern.MatchString(c.ReopenTestMR) {
		return c, errors.New("REOPEN_TEST_MR must use repo_id:iid, for example 316:588")
	}
	if v := getenv("TELEGRAM_MESSAGE_THREAD_ID"); v != "" {
		var err error
		if c.ThreadID, err = strconv.ParseInt(v, 10, 64); err != nil {
			return c, errors.New("TELEGRAM_MESSAGE_THREAD_ID must be an integer")
		}
	}
	return c, nil
}
