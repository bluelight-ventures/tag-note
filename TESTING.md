# Testing TagNote

## Test Mode

TagNote supports a test mode that creates a built-in test user on startup.

### Enable test mode

Set the `TAGNOTE_TEST_MODE` environment variable to `1`:

```bash
# Via docker compose
TAGNOTE_TEST_MODE=1 docker compose up --build

# Or add to .env file
echo "TAGNOTE_TEST_MODE=1" >> .env
docker compose up --build
```

### Test credentials

| Field    | Value           |
|----------|-----------------|
| Email    | `test@test.com` |
| Password | `testpass123`   |

### Get a test token (for CLI or API testing)

```bash
# Login and get a JWT token
curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@test.com","password":"testpass123"}' | jq -r '.token'
```

### CLI testing with the test account

```bash
# Get token and export it
export TAGNOTE_TOKEN=$(curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@test.com","password":"testpass123"}' | jq -r '.token')

# Now CLI tools are authenticated
docker compose exec tagnote tagnote-add -t test "Hello from test user"
docker compose exec tagnote tagnote-read -t test
docker compose exec tagnote tagnote-logs -t test
```

Or use the interactive login tool inside the container:

```bash
docker compose exec tagnote tagnote-login
# Enter: test@test.com / testpass123
# Copy the exported TAGNOTE_TOKEN
```

### Web UI testing

1. Start the server with `TAGNOTE_TEST_MODE=1`
2. Open `http://localhost:3777`
3. Log in with `test@test.com` / `testpass123`

### API testing examples

```bash
TOKEN=$(curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@test.com","password":"testpass123"}' | jq -r '.token')

# Create a note
curl -X POST http://localhost:3777/api/v1/notes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"Test note","tags":["test"]}'

# List notes
curl -H "Authorization: Bearer $TOKEN" http://localhost:3777/api/v1/notes

# Verify 401 without token
curl http://localhost:3777/api/v1/notes
# Should return: {"error":"missing authorization header"}
```

## Environment Variables

| Variable            | Description                                          | Default                    |
|---------------------|------------------------------------------------------|----------------------------|
| `JWT_SECRET`        | Secret key for signing JWT tokens                    | `tagnote-dev-secret`       |
| `TAGNOTE_TEST_MODE` | Set to `1` to create the test user on startup        | `0`                        |
| `TAGNOTE_TOKEN`     | JWT token for CLI authentication                     | (none)                     |
| `TAGNOTE_URL`       | Server URL for CLI tools                             | `http://localhost:3000`    |

## Legacy Data

Existing data (created before multi-user auth) is owned by a placeholder user
(`00000000000000000000000000` / `legacy@placeholder.local`). This data is not
visible to new accounts. To migrate legacy data to your account, ask for a
data migration after creating your account.
