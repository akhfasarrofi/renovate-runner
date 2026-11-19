# Security policy

Please do not open a public issue for security problems. Report them privately through the
repository's security advisory feature (GitHub: Security → Report a vulnerability) or by
contacting the maintainers directly.

Scope notes for deployers:

- The notifier must run behind an HTTPS reverse proxy; it authenticates webhooks with a shared secret (HMAC on GitHub and Bitbucket, secret token or signing token on GitLab) and never logs it.
- Keep the Renovate token and the webhook admin token out of the notifier `.env`.
