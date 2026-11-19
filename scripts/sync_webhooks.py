"""Add/update MR webhooks on RENOVATE_TEST_REPOSITORY (one project), else on every
project in RENOVATE_TARGET_REPOSITORIES (GitLab groups).

Reads variables from the shell, falling back to .env in the repo root.
Requires a GitLab token with Maintainer access to each project in those groups.
Only hooks with the exact configured URL are managed; no hooks are deleted.
"""

import json
import os
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen

from smoke_test import read_env


def groups(value):
    names = [name.strip() for name in value.replace("\n", ",").split(",") if name.strip()]
    if not names or len(names) != len(set(names)):
        raise ValueError("RENOVATE_TARGET_REPOSITORIES must list unique GitLab group names")
    return names


def snippet(raw):
    return raw[:200].decode(errors="replace").replace("\n", " ") or "<empty body>"


def api(method, url, token, fields=None):
    body = urlencode(fields).encode() if fields is not None else None
    request = Request(url, data=body, method=method, headers={"PRIVATE-TOKEN": token})
    with urlopen(request, timeout=15) as response:
        raw = response.read()
        try:
            return json.loads(raw)
        except ValueError:
            raise ValueError(f"{method} {url} returned HTTP {response.status} "
                             f"({response.headers.get('Content-Type')}), not JSON: {snippet(raw)}")


def get_all(url, token):
    """Fetch every page of a GitLab list endpoint."""
    items, page = [], 1
    while True:
        batch = api("GET", f"{url}{'&' if '?' in url else '?'}per_page=100&page={page}", token)
        if not isinstance(batch, list):
            raise ValueError("Unexpected GitLab list response")
        items.extend(batch)
        if len(batch) < 100:
            return items
        page += 1


def projects_in(api_url, token, group_names):
    """Project paths of all active projects in the groups, subgroups included."""
    paths = []
    for name in group_names:
        url = f"{api_url.rstrip('/')}/groups/{quote(name, safe='')}/projects?include_subgroups=true&archived=false"
        paths += [project["path_with_namespace"] for project in get_all(url, token)]
    return sorted(set(paths))


def sync(api_url, token, webhook_url, secret, paths):
    for path in paths:
        url = f"{api_url.rstrip('/')}/projects/{quote(path, safe='')}/hooks"
        hooks = get_all(url, token)
        matches = [hook for hook in hooks if hook.get("url") == webhook_url]
        if len(matches) > 1:
            raise ValueError(f"Multiple hooks have the configured URL in {path}; resolve duplicates manually")
        fields = {
            "url": webhook_url,
            "token": secret,
            "merge_requests_events": "true",
            "push_events": "false",
        }
        if matches:
            api("PUT", url + f"/{matches[0]['id']}", token, fields)
            print(f"Updated MR webhook: {path}")
        else:
            api("POST", url, token, fields)
            print(f"Created MR webhook: {path}")


def main():
    # Same .env as the notifier; variables already exported in the shell win.
    env_file = Path(__file__).parents[1] / ".env"
    if env_file.exists():
        for name, value in read_env(env_file).items():
            os.environ.setdefault(name, value)
    needed = ("CI_API_V4_URL", "GITLAB_WEBHOOK_ADMIN_TOKEN", "NOTIFIER_WEBHOOK_URL",
              "WEBHOOK_SECRET")
    test_repository = os.environ.get("RENOVATE_TEST_REPOSITORY", "").strip()
    if not test_repository:
        needed += ("RENOVATE_TARGET_REPOSITORIES",)
    elif "/" not in test_repository:
        raise ValueError("RENOVATE_TEST_REPOSITORY must be a group/project path")
    missing = [name for name in needed if not os.environ.get(name)]
    if missing:
        raise ValueError("Missing: " + ", ".join(missing))
    webhook_url = os.environ["NOTIFIER_WEBHOOK_URL"]
    if not webhook_url.startswith("https://") or not webhook_url.endswith("/webhook"):
        raise ValueError("NOTIFIER_WEBHOOK_URL must be an HTTPS URL ending in /webhook")
    api_url, token = os.environ["CI_API_V4_URL"], os.environ["GITLAB_WEBHOOK_ADMIN_TOKEN"]
    paths = ([test_repository] if test_repository else
             projects_in(api_url, token, groups(os.environ["RENOVATE_TARGET_REPOSITORIES"])))
    if not paths:
        raise ValueError("No active projects found in the configured groups")
    sync(api_url, token, webhook_url, os.environ["WEBHOOK_SECRET"], paths)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, HTTPError, URLError) as error:
        if isinstance(error, HTTPError):
            print(f"GitLab API returned HTTP {error.code} for {error.geturl()}: {snippet(error.read())}",
                  file=sys.stderr)
        else:
            print(f"Webhook sync failed: {error}", file=sys.stderr)
        sys.exit(1)
