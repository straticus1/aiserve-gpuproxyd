# PgBouncer Setup Guide for aiserve-gpuproxyd

## Overview

This application is optimized to work with PgBouncer, a lightweight connection pooler for PostgreSQL. Using PgBouncer allows you to handle **10,000+ concurrent connections** while keeping PostgreSQL connections low.

## Why PgBouncer?

| Scenario | Without PgBouncer | With PgBouncer |
|----------|-------------------|----------------|
| Max concurrent users | 25-50 | 10,000+ |
| PostgreSQL connections | 25 | 10-20 |
| Application connections | 25 max | 200+ per instance |
| Connection overhead | High | Minimal |
| Scalability | Vertical only | Horizontal ready |

## Quick Start

### 1. Install PgBouncer

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install pgbouncer
```

**macOS (Homebrew):**
```bash
brew install pgbouncer
```

**Docker:**
```bash
docker run -d \
  --name pgbouncer \
  -p 6432:6432 \
  -e DATABASES_HOST=postgres \
  -e DATABASES_PORT=5432 \
  -e DATABASES_USER=postgres \
  -e DATABASES_PASSWORD=changeme \
  -e DATABASES_DBNAME=gpuproxy \
  -e PGBOUNCER_POOL_MODE=transaction \
  -e PGBOUNCER_MAX_CLIENT_CONN=10000 \
  -e PGBOUNCER_DEFAULT_POOL_SIZE=20 \
  edoburu/pgbouncer:latest
```

### 2. Configure PgBouncer

Edit `/etc/pgbouncer/pgbouncer.ini`:

```ini
[databases]
gpuproxy = host=localhost port=5432 dbname=gpuproxy user=postgres password=changeme

[pgbouncer]
# Connection pooling mode
# transaction = recommended for high connection count
# session = if you need prepared statements or session-level features
# statement = most aggressive pooling (rarely used)
pool_mode = transaction

# Connection limits
max_client_conn = 10000           # Maximum client connections
default_pool_size = 20            # PostgreSQL connections per database
reserve_pool_size = 5             # Emergency connections
reserve_pool_timeout = 5          # Seconds to wait for emergency connection

# Timeouts
server_idle_timeout = 300         # Close idle server connections after 5 min
server_lifetime = 3600            # Close server connections after 1 hour
server_connect_timeout = 15       # Timeout for connecting to PostgreSQL

# Listen address
listen_addr = 127.0.0.1
listen_port = 6432

# Authentication
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

# Logging
logfile = /var/log/pgbouncer/pgbouncer.log
pidfile = /var/run/pgbouncer/pgbouncer.pid

# Admin
admin_users = postgres
stats_users = postgres
```

### 3. Create User List

Edit `/etc/pgbouncer/userlist.txt`:

```
"postgres" "md5d8578edf8458ce06fbc5bb76a58c5ca4"
```

To generate the MD5 hash:
```bash
echo -n "changemepostgres" | md5sum
# Format: md5 + md5(password + username)
```

### 4. Start PgBouncer

```bash
# Ubuntu/Debian
sudo systemctl enable pgbouncer
sudo systemctl start pgbouncer
sudo systemctl status pgbouncer

# macOS
pgbouncer -d /usr/local/etc/pgbouncer.ini

# Check logs
tail -f /var/log/pgbouncer/pgbouncer.log
```

### 5. Configure aiserve-gpuproxyd

Update your `.env` file:

```bash
# Point to PgBouncer instead of PostgreSQL directly
DB_HOST=localhost
DB_PORT=6432                      # PgBouncer port (not 5432)
DB_USER=postgres
DB_PASSWORD=changeme
DB_NAME=gpuproxy
DB_SSLMODE=disable

# Increase client connections since PgBouncer handles pooling
DB_MAX_CONNS=200                  # High because PgBouncer pools efficiently
DB_MIN_CONNS=20
DB_MAX_CONN_LIFETIME=15m          # Shorter for transaction pooling
DB_MAX_CONN_IDLE_TIME=5m          # Shorter idle time

# Enable PgBouncer mode
DB_USE_PGBOUNCER=true
DB_PGBOUNCER_POOL_MODE=transaction
```

### 6. Verify Connection

```bash
# Connect to PgBouncer admin console
psql -h localhost -p 6432 -U postgres pgbouncer

# Show pools
SHOW POOLS;

# Show clients
SHOW CLIENTS;

# Show servers
SHOW SERVERS;

# Show stats
SHOW STATS;
```

## PgBouncer Pool Modes Explained

### Transaction Mode (Recommended)

```ini
pool_mode = transaction
```

**How it works:**
- PostgreSQL connection is assigned for the duration of a transaction
- Connection returned to pool immediately after `COMMIT` or `ROLLBACK`
- Different queries from same client may use different PostgreSQL connections

**Pros:**
- Highest connection reuse
- Best for web applications with short transactions
- Can handle 10,000+ clients with 10-20 PostgreSQL connections

**Cons:**
- **Cannot use prepared statements** across transactions
- **Cannot use session-level features** (temp tables, advisory locks, SET commands)
- **No LISTEN/NOTIFY support**

**Best for:** aiserve-gpuproxyd (REST API with short transactions)

### Session Mode

```ini
pool_mode = session
```

**How it works:**
- PostgreSQL connection assigned when client connects
- Connection held until client disconnects
- Same PostgreSQL connection for entire client session

**Pros:**
- Supports prepared statements
- Supports all PostgreSQL features
- Compatible with all client libraries

**Cons:**
- Lower connection reuse
- Can only pool disconnected clients
- Requires 1:1 mapping during session

**Best for:** Long-running connections, applications using prepared statements

### Statement Mode (Advanced)

```ini
pool_mode = statement
```

**How it works:**
- PostgreSQL connection returned to pool after each SQL statement
- Most aggressive pooling

**Cons:**
- **Very restrictive** - breaks most applications
- Cannot use multi-statement transactions
- Cannot use prepared statements

**Best for:** Read-only query routing, specialized use cases

## Connection Scaling Examples

### Example 1: Small Deployment (100 concurrent users)

**PgBouncer:**
```ini
max_client_conn = 200
default_pool_size = 10
reserve_pool_size = 2
```

**aiserve-gpuproxyd (.env):**
```bash
DB_MAX_CONNS=50
DB_MIN_CONNS=10
```

**Result:** 100 concurrent users → 10 PostgreSQL connections

### Example 2: Medium Deployment (10,000 concurrent users)

**PgBouncer:**
```ini
max_client_conn = 10000
default_pool_size = 20
reserve_pool_size = 5
```

**aiserve-gpuproxyd (.env):**
```bash
DB_MAX_CONNS=200
DB_MIN_CONNS=20
REDIS_POOL_SIZE=50
```

**Result:** 10,000 concurrent users → 20 PostgreSQL connections

### Example 3: Large Deployment (100,000 concurrent users)

**Architecture:**
- 10x aiserve-gpuproxyd instances behind load balancer
- 1x PgBouncer instance
- 1x PostgreSQL (or managed RDS)

**PgBouncer:**
```ini
max_client_conn = 20000  # All app instances combined
default_pool_size = 50
reserve_pool_size = 10
```

**aiserve-gpuproxyd (.env per instance):**
```bash
DB_MAX_CONNS=200         # Per instance
DB_MIN_CONNS=20
REDIS_POOL_SIZE=100
```

**Result:** 100,000 concurrent users → 50 PostgreSQL connections

## Monitoring PgBouncer

### Admin Console Commands

```sql
# Connect
psql -h localhost -p 6432 -U postgres pgbouncer

# Show connection pools
SHOW POOLS;
# cl_active = active client connections
# sv_active = active server connections
# sv_idle = idle server connections

# Show all databases
SHOW DATABASES;

# Show current configuration
SHOW CONFIG;

# Show statistics
SHOW STATS;
# total_xact_count = total transactions
# total_query_count = total queries
# total_received = bytes received
# total_sent = bytes sent

# Reload configuration
RELOAD;

# Pause all operations
PAUSE;

# Resume operations
RESUME;

# Disconnect all clients
SHUTDOWN;
```

### Monitoring Metrics

**Key metrics to watch:**
1. **cl_waiting** - Clients waiting for connection (should be 0)
2. **sv_active** - Active PostgreSQL connections (should be < default_pool_size)
3. **maxwait** - Maximum wait time for connection (should be < 1 second)
4. **avg_xact_time** - Average transaction time
5. **avg_query_time** - Average query time

**Alert thresholds:**
- `cl_waiting > 0` for more than 10 seconds → increase default_pool_size
- `avg_xact_time > 1000ms` → optimize queries
- `maxwait > 5000ms` → PostgreSQL overloaded or too few connections

## Troubleshooting

### Problem: "prepared statement does not exist"

**Cause:** Using transaction mode with prepared statements

**Solution 1:** Switch to session mode
```ini
pool_mode = session
```

**Solution 2:** Disable prepared statements in your app
```bash
DB_PGBOUNCER_POOL_MODE=transaction
# pgx automatically disables prepared statements when detecting transaction pooling
```

### Problem: "temporary tables are not supported"

**Cause:** Using transaction mode with temp tables

**Solution:** Switch to session mode or refactor to use permanent tables

### Problem: "too many clients"

**Cause:** max_client_conn reached

**Solution:** Increase max_client_conn
```ini
max_client_conn = 20000
```

### Problem: High wait times (maxwait)

**Cause:** Not enough PostgreSQL connections

**Solution:** Increase default_pool_size
```ini
default_pool_size = 50
```

### Problem: PgBouncer won't start

**Check:**
1. PostgreSQL is running: `pg_isready -h localhost -p 5432`
2. User credentials correct in `userlist.txt`
3. Syntax in `pgbouncer.ini`
4. Logs: `tail -f /var/log/pgbouncer/pgbouncer.log`

## Best Practices

1. **Always use transaction mode** for REST APIs (like aiserve-gpuproxyd)
2. **Set default_pool_size = 2-3x CPU cores** on PostgreSQL server
3. **Monitor cl_waiting** - should always be 0 under normal load
4. **Use connection timeouts** - set reasonable server_idle_timeout
5. **Enable health checks** in your app (DB_HEALTH_CHECK_PERIOD=1m)
6. **Use multiple PgBouncer instances** for redundancy in production
7. **Run PgBouncer on same host** as PostgreSQL for lowest latency
8. **Use read replicas** with PgBouncer for read-heavy workloads

## Performance Tuning

### PostgreSQL Settings for PgBouncer

Edit `/etc/postgresql/*/main/postgresql.conf`:

```ini
# Connections
max_connections = 100              # Keep low (PgBouncer handles clients)
superuser_reserved_connections = 3

# Memory (per connection)
shared_buffers = 256MB             # 25% of RAM
effective_cache_size = 1GB         # 50% of RAM
work_mem = 4MB                     # RAM / max_connections / 4
maintenance_work_mem = 64MB

# Checkpointing
checkpoint_timeout = 10min
checkpoint_completion_target = 0.9

# Logging
log_min_duration_statement = 1000  # Log queries > 1 second
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
```

### OS Tuning

```bash
# Increase file descriptor limits
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# TCP tuning
sysctl -w net.core.somaxconn=4096
sysctl -w net.ipv4.tcp_max_syn_backlog=4096
sysctl -w net.ipv4.ip_local_port_range="1024 65535"
```

## Production Checklist

- [ ] PgBouncer installed and running
- [ ] `pool_mode = transaction` configured
- [ ] `default_pool_size` set to 2-3x PostgreSQL CPU cores
- [ ] `max_client_conn` set to expected concurrent users
- [ ] Authentication configured (auth_type, auth_file)
- [ ] Application configured to connect to PgBouncer port (6432)
- [ ] `DB_USE_PGBOUNCER=true` in application .env
- [ ] `DB_MAX_CONNS` increased (200+ for PgBouncer)
- [ ] Monitoring setup (SHOW POOLS, SHOW STATS)
- [ ] Alerts configured for cl_waiting > 0
- [ ] Health checks enabled (DB_HEALTH_CHECK_PERIOD=1m)
- [ ] Connection timeouts configured
- [ ] Logs configured and rotating
- [ ] Backup PgBouncer instance for redundancy
- [ ] Load tested with expected connection count

## References

- [PgBouncer Official Documentation](https://www.pgbouncer.org/usage.html)
- [PgBouncer GitHub](https://github.com/pgbouncer/pgbouncer)
- [PostgreSQL Connection Pooling Guide](https://www.postgresql.org/docs/current/runtime-config-connection.html)
