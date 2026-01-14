# IP Access Control

**Production-Critical Security Feature** - Prevent unauthorized access to GPU and AI resources by restricting API/gRPC access to specific IP addresses or ranges per user account.

## Overview

IP Access Control provides multi-layered protection against unauthorized access attempts by:

- **Per-account allowlists** (whitelist) - Only specific IPs can access
- **Per-account denylists** (blacklist) - Block specific IPs from accessing
- **CIDR range support** - Block/allow entire network ranges (e.g., `192.168.1.0/24`)
- **Temporary blocks** - Auto-expiring denylist entries
- **Full audit logging** - Track all access attempts for security monitoring
- **REST API management** - Users manage their own IP lists via API
- **CLI management** - Administrators can manage IP lists via command-line tool
- **gRPC support** - IP filtering works on both HTTP/REST and gRPC protocols

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Client Request                          │
│               (HTTP/REST or gRPC)                           │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Authentication       │
         │  (JWT or API Key)     │
         └───────────┬───────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  IP Access Control    │◄────────┐
         │  Middleware           │         │
         └───────────┬───────────┘         │
                     │                     │
          ┌──────────┴──────────┐         │
          │                     │         │
          ▼                     ▼         │
    ┌─────────┐          ┌──────────┐    │
    │ Deny-   │          │ Allow-   │    │
    │ list    │          │ list     │    │
    │ Check   │          │ Check    │    │
    └────┬────┘          └────┬─────┘    │
         │ BLOCKED            │ ALLOWED   │
         │                    │           │
         ▼                    ▼           │
    ┌────────────────────────────────┐   │
    │   Audit Log (if enabled)       │───┘
    └────────────────────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Protected Resource   │
         │  (GPU, AI, etc.)      │
         └───────────────────────┘
```

## Modes

IP Access Control supports 4 modes:

| Mode | Behavior |
|------|----------|
| `disabled` | No IP filtering (default) |
| `allowlist` | Only IPs in allowlist can access |
| `denylist` | All IPs except those in denylist can access |
| `strict` | Both allowlist AND denylist enforced (denylist takes priority) |

## Database Schema

### Tables

**ip_access_config** - Per-user configuration
```sql
- mode: disabled|allowlist|denylist|strict
- allowlist_enabled: boolean
- denylist_enabled: boolean
- block_on_no_match: boolean (block if IP not in allowlist)
- audit_log_enabled: boolean
```

**ip_allowlist** - Allowed IPs/ranges per user
```sql
- user_id: UUID
- ip_address: VARCHAR(45) - Exact IP (e.g., "203.0.113.5")
- ip_range: CIDR - Network range (e.g., "192.168.1.0/24")
- description: TEXT
- is_active: BOOLEAN
```

**ip_denylist** - Blocked IPs/ranges per user
```sql
- user_id: UUID
- ip_address: VARCHAR(45)
- ip_range: CIDR
- reason: TEXT
- is_active: BOOLEAN
- expires_at: TIMESTAMP (NULL = never expires)
```

**ip_access_log** - Audit trail
```sql
- user_id: UUID
- ip_address: VARCHAR(45)
- action: VARCHAR(20) - "allow", "deny", "check"
- result: VARCHAR(20) - "allowed", "blocked"
- reason: TEXT
- endpoint: TEXT
- method: VARCHAR(10)
- user_agent: TEXT
- created_at: TIMESTAMP
```

## REST API

All endpoints require authentication (`Authorization: Bearer <token>` or `X-API-Key: <key>`).

### Configuration

**Get Config**
```bash
GET /api/v1/ip-access/config
```

**Update Config**
```bash
PUT /api/v1/ip-access/config
Content-Type: application/json

{
  "mode": "allowlist",
  "allowlist_enabled": true,
  "denylist_enabled": true,
  "block_on_no_match": true,
  "audit_log_enabled": true
}
```

### Allowlist Management

**List Allowlist**
```bash
GET /api/v1/ip-access/allowlist
```

**Add to Allowlist**
```bash
POST /api/v1/ip-access/allowlist
Content-Type: application/json

{
  "ip_address": "203.0.113.5",
  "ip_range": "203.0.113.0/24",  // optional CIDR
  "description": "Office IP"
}
```

**Remove from Allowlist**
```bash
DELETE /api/v1/ip-access/allowlist/{id}
```

### Denylist Management

**List Denylist**
```bash
GET /api/v1/ip-access/denylist
```

**Add to Denylist**
```bash
POST /api/v1/ip-access/denylist
Content-Type: application/json

{
  "ip_address": "192.168.1.100",
  "ip_range": "192.168.1.0/24",  // optional CIDR
  "reason": "Suspicious activity",
  "expires_at": "2024-12-31T23:59:59Z"  // optional expiration
}
```

**Remove from Denylist**
```bash
DELETE /api/v1/ip-access/denylist/{id}
```

### Testing & Audit

**Check IP Access**
```bash
POST /api/v1/ip-access/check
Content-Type: application/json

{
  "ip_address": "203.0.113.5"
}

Response:
{
  "allowed": true,
  "reason": "IP in allowlist",
  "match_type": "allowlist"
}
```

**View Access Log**
```bash
GET /api/v1/ip-access/log?limit=100
```

## CLI Tool (`ipctl`)

The `ipctl` command-line tool allows administrators to manage IP access control for any user.

### Build & Install

```bash
# Build
go build -o ipctl cmd/ipctl/main.go

# Install globally
sudo mv ipctl /usr/local/bin/
```

### Configuration Management

```bash
# View current config
ipctl config get --user-email user@example.com

# Set mode
ipctl config set --user-email user@example.com --mode allowlist

# Enable features
ipctl config enable-allowlist --user-email user@example.com
ipctl config enable-denylist --user-email user@example.com
ipctl config enable-audit --user-email user@example.com
```

### Allowlist Management

```bash
# List allowlist entries
ipctl allowlist list --user-email user@example.com

# Add single IP
ipctl allowlist add --user-email user@example.com --ip 203.0.113.5 --description "Office IP"

# Add IP range
ipctl allowlist add --user-email user@example.com --ip 203.0.113.0 --range "203.0.113.0/24" --description "Office network"

# Remove IP
ipctl allowlist remove --user-email user@example.com --ip 203.0.113.5
```

### Denylist Management

```bash
# List denylist entries
ipctl denylist list --user-email user@example.com

# Block single IP permanently
ipctl denylist add --user-email user@example.com --ip 192.168.1.100 --reason "Brute force attack"

# Block IP for 24 hours
ipctl denylist add --user-email user@example.com --ip 192.168.1.100 --reason "Rate limit exceeded" --expires 24

# Block entire network
ipctl denylist add --user-email user@example.com --ip 192.168.1.0 --range "192.168.1.0/24" --reason "Suspicious network"

# Remove block
ipctl denylist remove --user-email user@example.com --ip 192.168.1.100
```

### Testing & Monitoring

```bash
# Check if IP would be allowed
ipctl check --user-email user@example.com --ip 203.0.113.5

# View access logs
ipctl log --user-email user@example.com --limit 50
```

## Usage Examples

### Example 1: Allowlist-Only Mode (Whitelist)

Lock down access so only specific IPs can connect:

```bash
# Set allowlist mode
curl -X PUT https://api.aiserve.farm/api/v1/ip-access/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "allowlist",
    "allowlist_enabled": true,
    "block_on_no_match": true
  }'

# Add your office IP
curl -X POST https://api.aiserve.farm/api/v1/ip-access/allowlist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ip_address": "203.0.113.5",
    "description": "Office static IP"
  }'

# Add home IP range
curl -X POST https://api.aiserve.farm/api/v1/ip-access/allowlist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ip_address": "198.51.100.0",
    "ip_range": "198.51.100.0/24",
    "description": "Home network"
  }'
```

**Result**: Only requests from `203.0.113.5` or `198.51.100.0/24` are allowed. All others are blocked.

### Example 2: Denylist Mode (Blacklist)

Block specific bad actors while allowing everyone else:

```bash
# Set denylist mode
curl -X PUT https://api.aiserve.farm/api/v1/ip-access/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "denylist",
    "denylist_enabled": true
  }'

# Block suspicious IP for 48 hours
curl -X POST https://api.aiserve.farm/api/v1/ip-access/denylist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ip_address": "192.168.1.100",
    "reason": "Multiple failed login attempts",
    "expires_at": "2024-12-31T23:59:59Z"
  }'

# Block entire malicious network permanently
curl -X POST https://api.aiserve.farm/api/v1/ip-access/denylist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ip_address": "192.0.2.0",
    "ip_range": "192.0.2.0/24",
    "reason": "Known bot network"
  }'
```

**Result**: All IPs except those in denylist are allowed.

### Example 3: Strict Mode (Both Lists)

Use allowlist for trusted IPs, but also maintain a denylist for known threats:

```bash
curl -X PUT https://api.aiserve.farm/api/v1/ip-access/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "mode": "strict",
    "allowlist_enabled": true,
    "denylist_enabled": true,
    "block_on_no_match": true
  }'
```

**Result**: Only allowlisted IPs are allowed, UNLESS they're also in the denylist (denylist takes priority).

### Example 4: Monitor Access Attempts

Enable audit logging to track who's trying to access your account:

```bash
# Enable audit logging
curl -X PUT https://api.aiserve.farm/api/v1/ip-access/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "audit_log_enabled": true
  }'

# View access log
curl https://api.aiserve.farm/api/v1/ip-access/log \
  -H "Authorization: Bearer $TOKEN"
```

## gRPC Support

IP Access Control works seamlessly with gRPC connections. The same allowlist/denylist rules apply.

### Client IP Extraction

The gRPC interceptor extracts client IP from:
1. **gRPC Peer Info** (`peer.FromContext`) - Direct connections
2. **X-Forwarded-For metadata** - Behind proxy/load balancer
3. **X-Real-IP metadata** - Nginx, Cloudflare

### Example gRPC Client

```go
conn, err := grpc.Dial("api.aiserve.farm:9090",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
client := pb.NewGPUProxyServiceClient(conn)

// Add API key to metadata
md := metadata.Pairs("x-api-key", "your-api-key")
ctx := metadata.NewOutgoingContext(context.Background(), md)

// If behind proxy, add X-Forwarded-For
md = metadata.Pairs(
    "x-api-key", "your-api-key",
    "x-forwarded-for", "203.0.113.5",
)
ctx = metadata.NewOutgoingContext(context.Background(), md)

// Make request - IP access control is enforced
resp, err := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{})
```

## Security Best Practices

### 1. Use Allowlist Mode for Production

**Most Secure**: Only allow known good IPs.

```bash
ipctl config set --user-email user@example.com --mode allowlist
ipctl config enable-allowlist --user-email user@example.com
```

### 2. Use CIDR Ranges for Networks

Don't add individual IPs if you control a network range:

```bash
# Bad: Adding 256 individual IPs
ipctl allowlist add --user-email user@example.com --ip 192.168.1.1
ipctl allowlist add --user-email user@example.com --ip 192.168.1.2
# ... (254 more)

# Good: Add entire /24 network
ipctl allowlist add --user-email user@example.com --ip 192.168.1.0 --range "192.168.1.0/24"
```

### 3. Enable Audit Logging

Always enable audit logging in production to track access attempts:

```bash
ipctl config enable-audit --user-email user@example.com
```

### 4. Use Temporary Blocks

Block suspicious IPs temporarily (auto-expire after X hours):

```bash
# Block for 24 hours
ipctl denylist add --user-email user@example.com --ip 192.168.1.100 --expires 24 --reason "Rate limit exceeded"
```

### 5. Regular Log Review

Monitor access logs regularly for suspicious patterns:

```bash
# Review last 100 access attempts
ipctl log --user-email user@example.com --limit 100 | grep "blocked"
```

### 6. Behind Cloudflare/Proxy

If using Cloudflare or a reverse proxy, ensure `X-Forwarded-For` or `CF-Connecting-IP` headers are passed:

```nginx
# Nginx example
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

## Performance

- **Optimized queries**: Uses PostgreSQL indexes for O(1) IP lookups
- **CIDR range matching**: Uses PostgreSQL `inet` and GiST indexes for fast network range queries
- **Minimal latency**: ~1-2ms overhead per request
- **Scales to millions**: Can handle millions of allowlist/denylist entries efficiently

### Indexes

```sql
-- Hot path optimization (covers 99% of queries)
CREATE INDEX idx_ip_allowlist_user_ip_active ON ip_allowlist(user_id, ip_address, is_active);
CREATE INDEX idx_ip_denylist_user_ip_active ON ip_denylist(user_id, ip_address, is_active);

-- CIDR range search (network blocks)
CREATE INDEX idx_ip_allowlist_range ON ip_allowlist USING gist(ip_range inet_ops);
CREATE INDEX idx_ip_denylist_range ON ip_denylist USING gist(ip_range inet_ops);
```

## Troubleshooting

### Issue: "Access denied: IP not in allowlist"

**Cause**: Allowlist mode enabled, but your IP isn't in the list.

**Solution**:
```bash
# Add your IP to allowlist
ipctl allowlist add --user-email your@email.com --ip YOUR_IP_ADDRESS

# Or disable allowlist mode
ipctl config set --user-email your@email.com --mode disabled
```

### Issue: IP Access Not Working Behind Proxy

**Cause**: Proxy not passing client IP headers.

**Solution**: Configure proxy to pass `X-Forwarded-For` or `X-Real-IP`:

```nginx
# Nginx
location / {
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

```apache
# Apache
RequestHeader set X-Forwarded-For "%{REMOTE_ADDR}s"
```

### Issue: Can't Access ipctl (CLI)

**Cause**: User not found in database.

**Solution**: Ensure user exists first:
```bash
# Register user via API first
curl -X POST https://api.aiserve.farm/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "...", "name": "..."}'

# Then use ipctl
ipctl config get --user-email user@example.com
```

## Migration Guide

### Enabling for Existing Users

```bash
# Enable for all users (admin script)
for email in $(psql -t -c "SELECT email FROM users"); do
  ipctl config set --user-email "$email" --mode disabled
  ipctl config enable-audit --user-email "$email"
done
```

### Zero-Downtime Rollout

1. **Phase 1**: Deploy with IP access control disabled (default)
2. **Phase 2**: Enable audit logging for all users (monitoring only)
3. **Phase 3**: Notify users about IP access control feature
4. **Phase 4**: Users opt-in by enabling allowlist/denylist via API/CLI

## Monitoring & Alerts

### Prometheus Metrics

```prometheus
# IP access blocks per user
ip_access_blocks_total{user_id="...",reason="..."}

# IP access checks per second
rate(ip_access_checks_total[5m])
```

### Alert Examples

```yaml
# Alert on high block rate
- alert: HighIPAccessBlockRate
  expr: rate(ip_access_blocks_total[5m]) > 10
  annotations:
    summary: "High IP access block rate for user {{ $labels.user_id }}"
```

## Support

For issues or questions:
- Email: support@afterdarksys.com
- GitHub: https://github.com/aiserve/gpuproxy/issues
- Docs: https://docs.aiserve.farm
