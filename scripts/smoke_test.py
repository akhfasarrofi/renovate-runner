"""Send a synthetic GitLab MR webhook to a locally running notifier.

Run from the repo root: python3 scripts/smoke_test.py --send
This sends one real test notification to each configured channel. The MR looks like a
Renovate security MR (package table + OSV advisory), so it shows the real message layout.
"""

import argparse
import base64
import hashlib
import hmac
import json
import secrets
import time
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen


# Same shape as Renovate's OSV security MR description.
DESCRIPTION = (
    "This MR contains the following updates:\n\n"
    "| Package | Change |\n|---|---|\n"
    "| [lodash](https://lodash.com/) | [`4.17.20` → `4.17.21`](https://renovatebot.com/diffs/npm/lodash/4.17.20/4.17.21) |\n\n"
    "---\n\n### Command Injection in lodash\n"
    "[CVE-2021-23337](https://nvd.nist.gov/vuln/detail/CVE-2021-23337) / "
    "[GHSA-35jh-r3h4-6jhm](https://github.com/advisories/GHSA-35jh-r3h4-6jhm)\n\n"
    "<details>\n<summary>More information</summary>\n\n#### Details\nSimulated advisory.\n\n"
    "#### Severity\n- CVSS Score: 7.2 / 10 (High)\n\n</details>\n"
)


def read_env(path):
    values = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        values[name.strip()] = value.strip().strip('"').strip("'")
    return values


def request(url, method="GET", body=None, headers=None):
    try:
        with urlopen(Request(url, data=body, headers=headers or {}, method=method), timeout=15) as response:
            return response.status
    except HTTPError as error:
        return error.code


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--send", action="store_true", help="send real messages to configured channels")
    parser.add_argument("--env", default=str(Path(__file__).parents[1] / ".env"), help="env file with the notifier settings")
    parser.add_argument("--url", default="http://127.0.0.1:8080", help="local notifier base URL")
    args = parser.parse_args()
    if not args.send:
        parser.error("add --send to confirm real test messages")

    env = read_env(Path(args.env))
    author_id = int(env["RENOVATE_AUTHOR"])
    signing = env.get("GITLAB_WEBHOOK_SIGNING_TOKEN", "")
    secret = env.get("WEBHOOK_SECRET", "")
    if bool(signing) == bool(secret):
        parser.error("set exactly one GitLab webhook credential in .env")

    health = request(args.url + "/healthz")
    print(f"GET /healthz: {health}")
    if health != 200:
        raise SystemExit("notifier is not running or reachable")

    iid = secrets.randbelow(1_000_000_000) + 1
    event = {
        "object_kind": "merge_request",
        "project": {
            "id": 999999999,
            "path_with_namespace": "local-test/renovate-notifier",
            "default_branch": "production",
        },
        "object_attributes": {
            "action": "open",
            "author_id": author_id,
            "iid": iid,
            "target_branch": "production",
            "title": "[LOCAL TEST] Update dependency lodash to v4.17.21 [SECURITY]",
            "url": f"https://gitlab.example.invalid/local-test/-/merge_requests/{iid}",
            "description": DESCRIPTION,
        },
    }
    body = json.dumps(event, separators=(",", ":")).encode("utf-8")
    headers = {"Content-Type": "application/json", "X-Gitlab-Event": "Merge Request Hook"}
    if signing:
        if not signing.startswith("whsec_"):
            parser.error("signing token must start with whsec_ (copy it from GitLab)")
        key = base64.b64decode(signing[6:], validate=True)
        message_id, timestamp = secrets.token_hex(16), str(int(time.time()))
        signed = message_id.encode() + b"." + timestamp.encode() + b"." + body
        digest = hmac.new(key, signed, hashlib.sha256).digest()
        headers.update({
            "webhook-id": message_id,
            "webhook-timestamp": timestamp,
            "webhook-signature": "v1," + base64.b64encode(digest).decode("ascii"),
        })
        bad_headers = {**headers, "webhook-signature": "v1,invalid"}
    else:
        headers["X-Gitlab-Token"] = secret
        bad_headers = {**headers, "X-Gitlab-Token": "incorrect-secret"}

    url = args.url + "/renovate/webhook"
    bad = request(url, "POST", body, bad_headers)
    print(f"Wrong credential: {bad} (expected 401)")
    if bad != 401:
        raise SystemExit("authentication check failed")
    good = request(url, "POST", body, headers)
    print(f"Valid bot MR: {good} (expected 200); test IID !{iid}")
    if good != 200:
        raise SystemExit("check notifier terminal logs; 401 = auth, 503 = delivery/database")
    again = request(url, "POST", body, headers)
    print(f"Same MR again: {again} (expected 200, no second messages)")
    if again != 200:
        raise SystemExit("duplicate delivery failed")
    print("Check for exactly one message for repo local-test/renovate-notifier in each configured channel.")


if __name__ == "__main__":
    main()
