# Contributing

Use Go 1.24 or newer.

Run the checks before opening a pull request:

```bash
go test ./...
go vet ./...
go build ./cmd/escrowd
```

Keep changes focused and use conventional commit messages.

Do not add source-code comments. Prefer clear names and small functions.

Do not commit credentials, generated binaries, job data, or local environment files.
