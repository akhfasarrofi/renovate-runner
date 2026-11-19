import unittest
from unittest.mock import patch

from sync_webhooks import groups, projects_in, sync


class SyncTests(unittest.TestCase):
    def test_group_list(self):
        self.assertEqual(groups(" frontend,backend\nplatform/web "), ["frontend", "backend", "platform/web"])
        with self.assertRaises(ValueError):
            groups("frontend,frontend")
        with self.assertRaises(ValueError):
            groups(" ")

    def test_projects_in_groups_dedupes_and_skips_archived(self):
        urls = []

        def fake_api(method, url, token, fields=None):
            urls.append(url)
            if "groups/frontend/" in url:
                return [{"path_with_namespace": "frontend/app"}, {"path_with_namespace": "frontend/web/ui"}]
            return [{"path_with_namespace": "frontend/web/ui"}, {"path_with_namespace": "backend/log"}]

        with patch("sync_webhooks.api", side_effect=fake_api):
            paths = projects_in("https://gitlab.example/api/v4", "t", ["frontend", "backend"])
        self.assertEqual(paths, ["backend/log", "frontend/app", "frontend/web/ui"])
        self.assertIn("include_subgroups=true&archived=false", urls[0])

    def test_create_and_update_without_duplicate(self):
        calls = []

        def fake_api(method, url, token, fields=None):
            calls.append((method, url, fields))
            if method == "GET":
                return [{"id": 5, "url": "https://notify.example/webhook"}] if "partner-v2" in url else []
            return {}

        with patch("sync_webhooks.api", side_effect=fake_api):
            sync("https://gitlab.example/api/v4", "admin-token",
                 "https://notify.example/webhook", "secret",
                 ["frontend/partner-v2", "backend/api"])
        self.assertEqual([call[0] for call in calls], ["GET", "PUT", "GET", "POST"])
        self.assertIn("frontend%2Fpartner-v2/hooks/5", calls[1][1])
        self.assertEqual(calls[3][2]["token"], "secret")
        self.assertEqual(calls[3][2]["push_events"], "false")


if __name__ == "__main__":
    unittest.main()
