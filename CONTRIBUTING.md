# Contributing

Thanks for taking the time to improve TagNote.

## Development

This project uses Go, SQLite, Fiber, and a vanilla JavaScript frontend embedded into the Go binary.

Run tests through Docker to match the project environment:

```bash
docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine go test ./...
```

Run the local development stack:

```bash
cp .env.example .env
docker compose up --build
```

The app is available at `http://localhost:3777/app`.

## Pull Requests

- Keep changes focused.
- Include tests for behavioral changes where practical.
- Run the Docker test command before opening a pull request.
- Do not commit local data, `.env` files, SQLite databases, uploads, or editor swap files.
