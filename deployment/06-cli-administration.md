# CLI Administration Commands

Administrative command-line interface for managing EK-1 authentication and system operations.

---

## Overview

EK-1 includes built-in CLI commands for administrative tasks like PIN management. These commands run the admin operation and exit, while running without flags starts the normal web server.

---

## Available Commands

### PIN Management

#### Check PIN Status
```bash
./ek1 -pin-status
```

**Output:**
```
✅ PIN is configured
# or
❌ PIN is not configured
```

#### Reset PIN
```bash
./ek1 -reset-pin
```

**Interactive prompt:**
```
⚠️  Are you sure you want to reset the PIN? (y/N): y
✅ PIN has been reset successfully
💡 You can now set up a new PIN via the frontend or API
```

#### Show Help
```bash
./ek1 -help
```

**Output:**
```
Usage of ./ek1:
  -pin-status
    	Show PIN configuration status
  -reset-pin
    	Reset the PIN authentication
```

---

## Docker Deployment

### Method 1: Execute in Running Container

If the EK-1 container is already running:

```bash
# Check PIN status
docker compose exec ek1 ek1 -pin-status

# Reset PIN (interactive)
docker compose exec -it ek1 ek1 -reset-pin

# Reset PIN (auto-confirm for scripts)
docker compose exec ek1 sh -c "echo 'y' | ek1 -reset-pin"
```

### Method 2: Run One-Shot Container

For rebuilding with latest changes:

```bash
# Rebuild and run CLI command
docker compose build ek1
docker compose run --rm ek1 ek1 -pin-status
docker compose run --rm -it ek1 ek1 -reset-pin
```

### Method 3: Direct Docker Commands

Without docker-compose:

```bash
# Find container name
docker ps | grep ek1

# Execute CLI command
docker exec ek1-ek1-1 ek1 -pin-status
docker exec -it ek1-ek1-1 ek1 -reset-pin
```

---

## Local Development

### Build and Run

```bash
# Build the application
go build ./cmd/ek1

# Test CLI commands
./ek1 -pin-status
./ek1 -reset-pin

# Start normal server (no flags)
./ek1
```

### Using Makefile

```bash
# Build locally
make build

# Run CLI commands
./ek1 -pin-status
./ek1 -reset-pin

# Start server
make run
```

---

## Usage Examples

### Basic PIN Management

```bash
# Check if PIN is configured
./ek1 -pin-status

# Reset PIN if forgotten
./ek1 -reset-pin
# Enter 'y' when prompted

# Verify PIN was reset
./ek1 -pin-status
```

### Docker Environment

```bash
# Check PIN status in production
docker compose exec ek1 ek1 -pin-status

# Reset PIN in production
docker compose exec -it ek1 ek1 -reset-pin

# Automated script for PIN reset
docker compose exec ek1 sh -c "echo 'y' | ek1 -reset-pin"
```

### CI/CD Integration

```bash
#!/bin/bash
# reset-pin.sh - Automated PIN reset script

echo "Resetting EK-1 PIN..."
docker compose exec ek1 sh -c "echo 'y' | ek1 -reset-pin"

if [ $? -eq 0 ]; then
    echo "PIN reset successful"
    exit 0
else
    echo "PIN reset failed"
    exit 1
fi
```

---

## Security Considerations

### PIN Reset Security

- **Reset requires physical/shell access** to the server or container
- **No network-based reset** - prevents remote attacks
- **Confirmation prompt** prevents accidental resets
- **Database operation is atomic** - either succeeds completely or fails

### Production Best Practices

1. **Restrict container access** - limit who can exec into containers
2. **Audit PIN resets** - log all administrative operations
3. **Backup before reset** - especially for production systems
4. **Immediate reconfiguration** - set new PIN immediately after reset

### Access Control

```bash
# Restrict CLI access to admin users only
sudo chown root:admin ./ek1
sudo chmod 750 ./ek1

# For Docker, restrict compose access
sudo chown root:docker-admin docker-compose.yml
sudo chmod 640 docker-compose.yml
```

---

## Troubleshooting

### Container Not Found

```bash
# List running containers
docker ps

# Start services if needed
docker compose up -d

# Use exact container name
docker exec -it <container_name> ek1 -pin-status
```

### Permission Denied

```bash
# Check file permissions
ls -la ./ek1

# Make executable if needed
chmod +x ./ek1
```

### Database Access Issues

```bash
# Check database file exists
ls -la ./ek1.db

# For Docker, check volume mount
docker compose exec ek1 ls -la /data/

# Verify database schema
docker compose exec ek1 sh -c "echo '.schema pin_auth' | sqlite3 /data/ek1.db"
```

### Build Issues

```bash
# Clean build
go clean -cache
go mod tidy
go build ./cmd/ek1

# Docker rebuild
docker compose build --no-cache ek1
```

---

## Database Details

### PIN Storage

The PIN is stored in the `pin_auth` table:

```sql
CREATE TABLE pin_auth (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    pin_hash   TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
```

### Manual Database Operations

**Check PIN directly:**
```bash
sqlite3 ek1.db "SELECT id, pin_hash != '' as configured, datetime(updated_at, 'unixepoch') as last_updated FROM pin_auth;"
```

**Manual PIN clear:**
```bash
sqlite3 ek1.db "UPDATE pin_auth SET pin_hash = '', updated_at = unixepoch() WHERE id = 1;"
```

---

## Integration with Authentication System

### After PIN Reset

1. **PIN is cleared** from database
2. **Frontend shows setup screen** on next load
3. **API returns PIN not configured** for `/auth/pin/status`
4. **New PIN setup via** `/auth/pin/setup` endpoint

### API Endpoints

```bash
# Check PIN status via API
curl https://genesis.egokernel.com/auth/pin/status

# Set new PIN via API (after reset)
curl -X POST https://genesis.egokernel.com/auth/pin/setup \
  -H "Content-Type: application/json" \
  -d '{"pin":"1234"}'
```

### JWT Token Invalidation

After PIN reset:
- **Existing JWT tokens remain valid** until expiration
- **New login required** with new PIN
- **Consider restarting application** to clear any cached state

---

## Automation Scripts

### Health Check with PIN Status

```bash
#!/bin/bash
# health-check.sh

echo "=== EK-1 Health Check ==="

# Check if service is running
if docker compose ps ek1 | grep -q "Up"; then
    echo "✅ EK-1 service is running"

    # Check PIN configuration
    PIN_STATUS=$(docker compose exec ek1 ek1 -pin-status 2>/dev/null)
    echo "PIN Status: $PIN_STATUS"
else
    echo "❌ EK-1 service is not running"
    exit 1
fi
```

### Backup Before Reset

```bash
#!/bin/bash
# backup-and-reset.sh

# Backup database
echo "Creating database backup..."
docker compose exec ek1 cp /data/ek1.db "/data/ek1.db.backup.$(date +%Y%m%d_%H%M%S)"

# Reset PIN
echo "Resetting PIN..."
docker compose exec ek1 sh -c "echo 'y' | ek1 -reset-pin"

echo "Backup and reset complete"
```