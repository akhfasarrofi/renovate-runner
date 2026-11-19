# Contributing

Bug reports and pull requests are welcome.

## Setup

Go 1.25+ and Python 3 (for the webhook scripts).

```bash
go vet ./... && go test -race ./...
python3 -I -m unittest discover -s scripts -p 'test_*.py'
```

## Guidelines

- Keep changes small and add a test for new logic. Platform parsers live in `internal/platform/`; add a case to the matching `*_test.go`.
- Do not commit secrets or a real `.env`. Use fake values in tests.
- Adding a hosting platform means: implement `Platform` (`Authorized` + `Parse`) in `internal/platform/`, register it in `New`, allow it in `internal/config`, and document the webhook setup in the README.
- Payload shapes should come from the platform's official docs; say in the PR if you could not verify against a real webhook.

By contributing you agree your work is released under the MIT license.
