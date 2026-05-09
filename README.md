# Inspector

> Looking for Spanish documentation? Read README.es.md

Inspector is a Go-based developer tool to inspect, debug, and send HTTP, webhook, and WebSocket traffic. It provides a real-time web dashboard where you can receive inbound requests, inspect headers and payloads, and send outbound requests with full history.

---

## Features

- HTTP/Webhook receiver: create custom slug endpoints at /in/:slug and capture any inbound request.
- WebSocket receiver: each endpoint also exposes ws://.../in/:slug/ws.
- Real-time dashboard: live updates using Server-Sent Events (SSE).
- Request history: filterable and paginated list of captured inbound traffic.
- Request diff: compare two captured requests at /requests/diff.
- Endpoint management: create, edit, delete endpoints and clear endpoint history.
- Mock Rules: conditional mock responses with endpoint or global scope, deterministic priority, enable/disable, and inline editing.
- Global exclusion support: global mock rules can exclude specific endpoints.
- HTTP sender: compose and send outbound HTTP requests with response trace.
- WebSocket client: connect to any WS server, send messages, view stream in real time.
- Advanced sent history: filters by type, method, status, text, date ranges.
- Sensitive data redaction: redact configured headers/fields before persistence.
- Outbound failure alerts: optional webhook alerts for critical outbound failures.
- Session authentication: form login + session cookie.
- Auto purge: automatic cleanup based on max_requests.
- No CGO: embedded SQLite with pure Go driver.

---

## Tech Stack

- Web framework: Gin
- Templates: Go HTML templates + gin-contrib/multitemplate
- Database: SQLite via GORM + glebarez/sqlite
- WebSocket: gorilla/websocket
- Realtime: native SSE
- Config: YAML
- Frontend: Tailwind CSS (CDN), HTMX, Lucide icons

---

## Requirements

- Go 1.21+
- No CGO and no external C libraries required

---

## Installation and Run

1) Clone the repository

git clone <repo-url> inspector
cd inspector

2) Install dependencies

go mod tidy

3) Configure

cp config.example.yaml config.yaml

Important: config.yaml is gitignored because it contains credentials. Do not commit it.

4) Run

Development:
go run main.go

Build:
go build -o inspector .
./inspector

Custom config file:
go run main.go my-config.yaml

Default URL: http://localhost:9090

---

## Docker

Production:

docker compose up -d --build

docker compose ps
docker compose logs -f inspector

Development:

docker compose -f docker-compose.dev.yml up -d

Optional Make commands:

make docker-build
make docker-up
make docker-down
make docker-dev-up
make docker-dev-down

---

## Public Exposure (Tunnels)

You can expose Inspector with tools like ngrok, localtunnel, or Cloudflare Tunnel.

Start Inspector locally first:

go run main.go

Then expose port 9090.

Examples:

- ngrok: ngrok http 9090
- localtunnel: lt --port 9090
- cloudflared quick tunnel: cloudflared tunnel --url http://127.0.0.1:9090

Public endpoint examples:

- HTTP: https://<public-host>/in/<slug>
- WS: wss://<public-host>/in/<slug>/ws

---

## Configuration

Main settings live in config.yaml:

- server.host, server.port
- auth.username, auth.password
- database.path
- settings.max_requests
- settings.max_request_body_bytes
- settings.max_response_body_bytes
- settings.cleanup_interval_seconds
- settings.session_ttl_hours
- settings.allowed_ws_origins
- settings.redaction_enabled
- settings.redaction_headers
- settings.redaction_fields
- settings.alert_webhook_url
- settings.alert_min_sent_status
- settings.alert_on_sent_error

You can also override with env vars:

- INSPECTOR_SERVER_HOST
- INSPECTOR_SERVER_PORT
- INSPECTOR_AUTH_USERNAME
- INSPECTOR_AUTH_PASSWORD
- INSPECTOR_DATABASE_PATH
- INSPECTOR_SETTINGS_MAX_REQUESTS
- INSPECTOR_SETTINGS_MAX_REQUEST_BODY_BYTES
- INSPECTOR_SETTINGS_MAX_RESPONSE_BODY_BYTES
- INSPECTOR_SETTINGS_CLEANUP_INTERVAL_SECONDS
- INSPECTOR_SETTINGS_SESSION_TTL_HOURS
- INSPECTOR_SETTINGS_ALLOWED_WS_ORIGINS
- INSPECTOR_SETTINGS_REDACTION_ENABLED
- INSPECTOR_SETTINGS_REDACTION_HEADERS
- INSPECTOR_SETTINGS_REDACTION_FIELDS
- INSPECTOR_SETTINGS_ALERT_WEBHOOK_URL
- INSPECTOR_SETTINGS_ALERT_MIN_SENT_STATUS
- INSPECTOR_SETTINGS_ALERT_ON_SENT_ERROR

Operational vars:

- INSPECTOR_ENV
- INSPECTOR_ALLOW_DEFAULT_AUTH

---

## Usage

Login:

- Open /login
- Authenticate with configured credentials
- Session cookie: inspector_session

Create an endpoint:

- Go to Endpoints
- Fill name, slug, optional description
- Submit

Inbound routes:

- HTTP: /in/<slug>
- WS: /in/<slug>/ws

HTTP sender:

- Go to Send
- Choose method, URL, headers, body
- Send and inspect response

Mock Rules:

- Central page: /mocks
- Endpoint page: quick actions and modal editing
- Scopes: endpoint or global
- Matching by method/path/query/headers/body
- Response controls: status, headers, body, delay, active flag

Precedence:

1. Lower priority number first
2. Endpoint scope wins over global on tie
3. Lower ID on tie
4. Global rule is skipped if endpoint is excluded
5. Endpoint static response as fallback

---

## Main Routes

Public:

- ANY /in/:slug
- GET /in/:slug/ws
- GET /healthz
- GET /readyz
- GET /login
- POST /login
- GET /logout

Authenticated:

- GET /dashboard
- GET /requests
- GET /requests/diff
- GET /requests/:id
- GET /endpoints
- POST /endpoints
- PUT/POST /endpoints/:id
- DELETE /endpoints/:id
- POST /endpoints/:id/clear
- GET /mocks
- GET /mocks/global
- POST /mocks
- PUT/POST /mocks/:mockId
- DELETE /mocks/:mockId
- POST /mocks/:mockId/toggle
- GET /send
- POST /send/http
- GET /send/history
- GET /send/history/:id
- GET /send/ws-client
- GET /send/ws-proxy
- GET /events
- GET /events/ws
- GET /events/poll

---

## Project Structure

- main.go: app entrypoint and router
- internal/config: config loading
- internal/models: data models
- internal/storage: DB init and cleanup
- internal/broadcaster: SSE hub
- internal/middleware: auth/security middleware
- internal/handlers: HTTP handlers
- internal/i18n: i18n bundle, middleware, locales
- web/templates: HTML views

---

## License

MIT
