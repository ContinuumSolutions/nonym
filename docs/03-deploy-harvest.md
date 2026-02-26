# Deploying the Social Harvest Scanner

The harvest scanner (`scripts/harvest/scan.go`) identifies Hidden Value in
your professional and social networks: outstanding social debts, Ghost-Agreement
opportunities, and passive revenue signals.

In **Phase 1** it runs on a static contact list (demo mode).
In **Phase 2+** it connects to live API integrations.

---

## Phase 1 — Local Demo

No external dependencies. Runs immediately.

```bash
# Run directly
go run ./scripts/harvest/scan.go

# Build a standalone binary
go build -o harvest ./scripts/harvest/scan.go
./harvest
```

**Output:**
- Social debts ranked by estimated USD value
- Ghost-Agreement opportunities (>= 95% solution overlap)
- Total estimated network value

---

## Phase 2 — Live API Integrations

Replace the demo `contacts` slice in `scan.go` with real data sources.

### LinkedIn API

```bash
# Register a LinkedIn Developer App at:
# https://www.linkedin.com/developers/apps

export LINKEDIN_CLIENT_ID="your_client_id"
export LINKEDIN_CLIENT_SECRET="your_client_secret"
export LINKEDIN_ACCESS_TOKEN="your_access_token"
```

Fetch your connection graph:

```bash
curl -H "Authorization: Bearer $LINKEDIN_ACCESS_TOKEN" \
  "https://api.linkedin.com/v2/connections?q=viewer&count=500" \
  | jq '.elements[]'
```

Map the response into `ContactRecord` structs and pass them to `NewScanner()`.

### GitHub Activity Graph

```bash
export GITHUB_TOKEN="ghp_your_token"

# Fetch your interaction history (PRs reviewed, issues helped, etc.)
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  "https://api.github.com/users/YOUR_USERNAME/events" \
  | jq '.[].type'
```

### Gmail / Outlook (Email Graph)

Use the Gmail API to count sent vs. received emails per contact:

```bash
# Gmail API — list recent threads
curl -H "Authorization: Bearer $GOOGLE_ACCESS_TOKEN" \
  "https://gmail.googleapis.com/gmail/v1/users/me/threads?maxResults=500"
```

Convert thread counts into `FavorsReceived` / `FavorsGiven` metrics.

---

## Phase 2 — Running as a Scheduled Job

### Cron (Linux)

Run the scanner daily at 06:00 and log results:

```bash
# Add to crontab
crontab -e

# Entry:
0 6 * * * /path/to/harvest >> /var/log/ek1-harvest.log 2>&1
```

### Systemd Service

```ini
# /etc/systemd/system/ek1-harvest.service
[Unit]
Description=EK-1 Social Harvest Scanner
After=network.target

[Service]
Type=oneshot
ExecStart=/path/to/harvest
StandardOutput=append:/var/log/ek1-harvest.log
StandardError=append:/var/log/ek1-harvest.log
Environment="LINKEDIN_ACCESS_TOKEN=your_token"
Environment="GITHUB_TOKEN=your_token"

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/ek1-harvest.timer
[Unit]
Description=Run EK-1 Harvest Scanner daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

```bash
sudo systemctl enable --now ek1-harvest.timer
sudo systemctl list-timers | grep ek1
```

### Docker

```dockerfile
# Dockerfile.harvest
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY scripts/harvest/scan.go scripts/harvest/scan.go
RUN go build -o harvest ./scripts/harvest/scan.go

FROM alpine:latest
COPY --from=builder /app/harvest /usr/local/bin/harvest
ENV LINKEDIN_ACCESS_TOKEN=""
ENV GITHUB_TOKEN=""
CMD ["harvest"]
```

```bash
docker build -f Dockerfile.harvest -t ek1-harvest .

# Run once
docker run --rm \
  -e LINKEDIN_ACCESS_TOKEN="$LINKEDIN_ACCESS_TOKEN" \
  -e GITHUB_TOKEN="$GITHUB_TOKEN" \
  ek1-harvest

# Schedule with Docker + cron or Kubernetes CronJob
```

---

## Phase 3 — On-Chain Favor Tokens

Once the Anchor program is live (see [`02-deploy-anchor.md`](./02-deploy-anchor.md)),
the scanner can convert Social Debt into on-chain Blind Favor Tokens:

1. Scanner identifies a contact with net favors owed > 3.
2. Calls `initialize_kernel` for the contact (if not already on-chain).
3. Calls `create_escrow` to lock a micro-payment representing the debt.
4. Sends the contact a signed request: settle the escrow (return the favor)
   or have their reputation flagged via `flag_bad_faith`.

This converts soft social leverage into cryptographically enforced commitments.

---

## Output Format

The scanner prints to stdout. To export as JSON for downstream processing:

```bash
# Redirect to a file
./harvest > harvest-$(date +%Y%m%d).json

# Pipe into the Go brain for automatic action queuing (Phase 2)
./harvest | go run ./cmd/ek1/ --harvest-mode
```
