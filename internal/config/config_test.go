package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	base := map[string]string{"RENOVATE_AUTHOR_ID": "42", "GITLAB_WEBHOOK_SECRET": "x", "TELEGRAM_BOT_TOKEN": "t", "TELEGRAM_CHAT_ID": "1"}
	env := func(over map[string]string) func(string) string {
		return func(k string) string {
			if v, ok := over[k]; ok {
				return v
			}
			return base[k]
		}
	}
	if c, err := Load(env(nil)); err != nil || c.TelegramToken != "t" || c.ThreadID != 0 {
		t.Fatalf("valid config rejected: %v", err)
	}
	if c, err := Load(env(map[string]string{"RENOVATE_PLATFORM": "github", "RENOVATE_AUTHOR": "renovate[bot]", "WEBHOOK_SECRET": "y", "GITLAB_WEBHOOK_SECRET": ""})); err != nil || c.Author != "renovate[bot]" || c.Secret != "y" {
		t.Fatalf("github config rejected: %v", err)
	}
	for _, bad := range []map[string]string{
		{"RENOVATE_AUTHOR_ID": ""},
		{"RENOVATE_AUTHOR_ID": "-1"},
		{"GITLAB_WEBHOOK_SECRET": ""},
		{"GITLAB_WEBHOOK_SIGNING_TOKEN": "whsec_x"},
		{"TELEGRAM_CHAT_ID": ""},
		{"TELEGRAM_BOT_TOKEN": "", "TELEGRAM_CHAT_ID": ""},
		{"SLACK_WEBHOOK_URL": "http://hooks.slack.com/x"},
		{"REOPEN_TEST_MR": "316"},
		{"TELEGRAM_MESSAGE_THREAD_ID": "abc"},
		{"RENOVATE_PLATFORM": "svn"},
		{"NOTIFY_LANG": "fr"},
		{"RENOVATE_PLATFORM": "github", "RENOVATE_AUTHOR_ID": "renovate[bot]", "GITLAB_WEBHOOK_SECRET": "", "GITLAB_WEBHOOK_SIGNING_TOKEN": "whsec_x"},
	} {
		if _, err := Load(env(bad)); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}
