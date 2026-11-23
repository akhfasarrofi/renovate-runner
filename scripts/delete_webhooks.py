"""Delete project webhooks whose URL equals the given one, on RENOVATE_TARGET_REPOSITORY.

Usage: python3 scripts/delete_webhooks.py <exact-webhook-url> [--yes]
Dry run unless --yes is passed. Only hooks with the exact URL are touched.
Same env as sync_webhooks.py (CI_API_V4_URL, GITLAB_WEBHOOK_ADMIN_TOKEN,
RENOVATE_TARGET_REPOSITORY), read from the shell, falling back to .env.
"""

import os
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

from smoke_test import read_env
from sync_webhooks import get_all, projects_in, snippet, targets


def main(webhook_url, apply):
    env_file = Path(__file__).parents[1] / ".env"
    if env_file.exists():
        for name, value in read_env(env_file).items():
            os.environ.setdefault(name, value)
    needed = ("CI_API_V4_URL", "GITLAB_WEBHOOK_ADMIN_TOKEN", "RENOVATE_TARGET_REPOSITORY")
    missing = [name for name in needed if not os.environ.get(name)]
    if missing:
        raise ValueError("Missing: " + ", ".join(missing))
    api_url, token = os.environ["CI_API_V4_URL"].rstrip("/"), os.environ["GITLAB_WEBHOOK_ADMIN_TOKEN"]
    projects, group_names = targets(os.environ["RENOVATE_TARGET_REPOSITORY"])
    failed = found = 0
    for path in projects or projects_in(api_url, token, group_names):
        hooks_url = f"{api_url}/projects/{quote(path, safe='')}/hooks"
        try:
            for hook in (h for h in get_all(hooks_url, token) if h.get("url") == webhook_url):
                found += 1
                if apply:
                    # 204 has no body, so api() (which parses JSON) cannot be used.
                    urlopen(Request(f"{hooks_url}/{hook['id']}", method="DELETE",
                                    headers={"PRIVATE-TOKEN": token}), timeout=15).close()
                print(f"{'Deleted' if apply else 'Would delete'} hook {hook['id']}: {path}")
        except HTTPError as error:
            print(f"FAILED {path}: HTTP {error.code} {snippet(error.read())}", file=sys.stderr)
            failed += 1
    print(f"{found} hook(s) {'deleted' if apply else 'found (dry run, pass --yes to delete)'}")
    if failed:
        raise ValueError(f"{failed} project(s) failed")


if __name__ == "__main__":
    args = [a for a in sys.argv[1:] if a != "--yes"]
    if len(args) != 1:
        sys.exit(__doc__)
    try:
        main(args[0], "--yes" in sys.argv)
    except (ValueError, HTTPError, URLError) as error:
        print(f"Webhook delete failed: {error}", file=sys.stderr)
        sys.exit(1)
