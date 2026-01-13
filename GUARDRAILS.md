# Guard Rails - Spending Control System

The Guard Rails feature provides configurable spending limits across multiple time windows to prevent out-of-control spending on GPU resources.

## Overview

Guard Rails tracks spending in real-time across 17 different time windows, from 5 minutes to 72 hours, and automatically blocks requests when configured spending limits are exceeded.

## Features

- **Multiple Time Windows**: Track spending across 17 different time periods
- **Real-time Enforcement**: Automatically blocks requests when limits are exceeded
- **Per-User Tracking**: Individual spending tracking for each user
- **Flexible Configuration**: Enable/disable specific time windows as needed
- **Admin Tools**: Command-line utilities for monitoring and management
- **API Integration**: REST API endpoints for programmatic access

## Supported Time Windows

| Window Name | Duration | Environment Variable |
|-------------|----------|---------------------|
| 5min | 5 minutes | GUARDRAILS_MAX_5MIN_RATE |
| 15min | 15 minutes | GUARDRAILS_MAX_15MIN_RATE |
| 30min | 30 minutes | GUARDRAILS_MAX_30MIN_RATE |
| 60min | 1 hour | GUARDRAILS_MAX_60MIN_RATE |
| 90min | 1.5 hours | GUARDRAILS_MAX_90MIN_RATE |
| 120min | 2 hours | GUARDRAILS_MAX_120MIN_RATE |
| 240min | 4 hours | GUARDRAILS_MAX_240MIN_RATE |
| 300min | 5 hours | GUARDRAILS_MAX_300MIN_RATE |
| 360min | 6 hours | GUARDRAILS_MAX_360MIN_RATE |
| 400min | 6.67 hours | GUARDRAILS_MAX_400MIN_RATE |
| 460min | 7.67 hours | GUARDRAILS_MAX_460MIN_RATE |
| 520min | 8.67 hours | GUARDRAILS_MAX_520MIN_RATE |
| 640min | 10.67 hours | GUARDRAILS_MAX_640MIN_RATE |
| 700min | 11.67 hours | GUARDRAILS_MAX_700MIN_RATE |
| 1440min | 24 hours | GUARDRAILS_MAX_1440MIN_RATE |
| 48h | 48 hours | GUARDRAILS_MAX_48H_RATE |
| 72h | 72 hours | GUARDRAILS_MAX_72H_RATE |

## Configuration

### Environment Variables

Add these to your `.env` file:

```bash
# Enable guard rails
GUARDRAILS_ENABLED=true

# Set spending limits (USD) - set to 0 to disable a specific window
GUARDRAILS_MAX_5MIN_RATE=10.00
GUARDRAILS_MAX_15MIN_RATE=25.00
GUARDRAILS_MAX_30MIN_RATE=50.00
GUARDRAILS_MAX_60MIN_RATE=100.00
GUARDRAILS_MAX_90MIN_RATE=150.00
GUARDRAILS_MAX_120MIN_RATE=200.00
GUARDRAILS_MAX_240MIN_RATE=400.00
GUARDRAILS_MAX_300MIN_RATE=500.00
GUARDRAILS_MAX_360MIN_RATE=600.00
GUARDRAILS_MAX_400MIN_RATE=650.00
GUARDRAILS_MAX_460MIN_RATE=700.00
GUARDRAILS_MAX_520MIN_RATE=800.00
GUARDRAILS_MAX_640MIN_RATE=1000.00
GUARDRAILS_MAX_700MIN_RATE=1100.00
GUARDRAILS_MAX_1440MIN_RATE=2000.00
GUARDRAILS_MAX_48H_RATE=4000.00
GUARDRAILS_MAX_72H_RATE=6000.00
```

### Example Configurations

#### Conservative Limits
```bash
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_60MIN_RATE=50.00     # $50/hour
GUARDRAILS_MAX_1440MIN_RATE=500.00  # $500/day
GUARDRAILS_MAX_72H_RATE=1000.00     # $1000/3 days
```

#### Development Environment
```bash
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_5MIN_RATE=1.00       # Quick protection
GUARDRAILS_MAX_60MIN_RATE=10.00     # Hourly limit
GUARDRAILS_MAX_1440MIN_RATE=50.00   # Daily limit
```

#### Production Environment
```bash
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_60MIN_RATE=500.00
GUARDRAILS_MAX_240MIN_RATE=1500.00
GUARDRAILS_MAX_1440MIN_RATE=5000.00
GUARDRAILS_MAX_72H_RATE=12000.00
```

## How It Works

1. **Request Interception**: Guard Rails middleware checks spending before allowing API requests
2. **Spending Calculation**: Current spending is retrieved from Redis for all configured time windows
3. **Limit Validation**: Spending + estimated cost is compared against configured limits
4. **Decision**: Request is either allowed or blocked with a 402 Payment Required status
5. **Recording**: After successful requests, actual costs are recorded to all time windows

### Data Storage

- Spending data is stored in Redis with automatic expiration
- Each time window has its own key with TTL matching the window duration
- No database storage required - all tracking is ephemeral

## API Usage

### Check Spending Status

Get current spending across all time windows:

```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/guardrails/spending
```

Response:
```json
{
  "user_id": "uuid",
  "timestamp": "2024-01-12T10:30:00Z",
  "window_spent": {
    "5min": 2.50,
    "60min": 45.00,
    "1440min": 750.00
  },
  "violations": []
}
```

### Check If Request Would Exceed Limits

```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"estimated_cost": 25.00}' \
  http://localhost:8080/api/v1/guardrails/spending/check
```

Response if allowed:
```json
{
  "allowed": true,
  "spent": {
    "60min": 45.00,
    "1440min": 750.00
  }
}
```

Response if exceeded (402 Payment Required):
```json
{
  "allowed": false,
  "violations": [
    "60min: $45.00 + $25.00 = $70.00 > $50.00 limit"
  ],
  "spent": {
    "60min": 45.00
  }
}
```

### Record Spending

Manually record spending (typically done automatically by the system):

```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"amount": 15.50}' \
  http://localhost:8080/api/v1/guardrails/spending/record
```

### Reset Spending

Reset spending tracking (admin only, use CLI tool):

```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"window_name": "60min"}' \
  http://localhost:8080/api/v1/guardrails/spending/reset
```

## Admin Commands

### View Guard Rails Status

```bash
./bin/aiserve-gpuproxy-admin guardrails-status
```

Output:
```
Guard Rails Configuration
=========================
Enabled: true

Spending Limits by Time Window:
--------------------------------
  5 minutes:                $10.00
  15 minutes:               $25.00
  60 minutes (1h):          $100.00
  1440 minutes (24h):       $2000.00

Total active limits: 4
```

### View User Spending

```bash
./bin/aiserve-gpuproxy-admin guardrails-spending user@example.com
```

Output:
```
Guard Rails Spending for John Doe (user@example.com)
========================================
User ID: uuid-here
Timestamp: 2024-01-12 10:30:00

Spending by Time Window:
------------------------
WINDOW    SPENT     STATUS
------    -----     ------
5min      $2.50     OK
15min     $8.75     OK
60min     $45.00    OK
1440min   $750.00   OK
```

### Reset User Spending

Reset all windows:
```bash
./bin/aiserve-gpuproxy-admin guardrails-reset user@example.com
```

Reset specific window:
```bash
./bin/aiserve-gpuproxy-admin guardrails-reset user@example.com 60min
```

## HTTP Response Headers

When guard rails are enabled, these headers are included in responses:

```
X-GuardRails-Enabled: true
X-GuardRails-5min: 2.50
X-GuardRails-60min: 45.00
X-GuardRails-1440min: 750.00
```

When limits are exceeded:
```
X-GuardRails-Exceeded: true
```

## Error Responses

### 402 Payment Required

When spending limits are exceeded:

```json
{
  "error": "Spending limit exceeded",
  "violations": [
    "60min: $50.00 + $25.00 = $75.00 > $50.00 limit",
    "1440min: $2000.00 + $25.00 = $2025.00 > $2000.00 limit"
  ],
  "spent": {
    "60min": 50.00,
    "1440min": 2000.00
  }
}
```

## Integration with GPU Usage

To automatically track spending from GPU usage, record costs after GPU operations:

```go
// After GPU usage
cost := calculateCost(duration, pricePerHour)
if err := guardRails.RecordSpending(ctx, userID, cost); err != nil {
    log.Printf("Failed to record spending: %v", err)
}
```

## Best Practices

1. **Start Conservative**: Begin with lower limits and adjust based on usage patterns
2. **Multiple Windows**: Configure limits at multiple time scales (hour, day, week)
3. **Monitor Regularly**: Use admin commands to track spending patterns
4. **Alert Integration**: Set up alerts when users approach limits
5. **Document Limits**: Inform users of configured spending limits
6. **Graceful Handling**: Provide clear error messages when limits are exceeded

## Architecture

```
┌─────────────┐
│   Request   │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────┐
│   Guard Rails Middleware    │
│  - Check spending limits    │
│  - Calculate violations     │
└──────┬──────────────────────┘
       │
       ▼
    ┌──────┐
    │Redis │ ◄── Spending data with TTL
    └──────┘
       │
       ▼
┌─────────────────┐
│  Allow/Block    │
│   Request       │
└─────────────────┘
```

## Performance Considerations

- **Redis Usage**: One key per user per time window
- **Memory**: Minimal - only active users tracked
- **Latency**: < 5ms overhead per request
- **Auto-Cleanup**: Keys expire automatically based on window duration
- **No Database**: All tracking in Redis for speed

## Troubleshooting

### Guard Rails Not Enforcing

1. Check that `GUARDRAILS_ENABLED=true` in `.env`
2. Verify Redis connection is working
3. Confirm middleware is applied to routes in `cmd/server/main.go`
4. Check that limits are configured (> 0)

### Spending Not Recording

1. Verify Redis connectivity
2. Check logs for errors
3. Ensure spending recording calls are in place
4. Verify user ID is being passed correctly

### False Positives

1. Check if time window limits are too restrictive
2. Verify spending calculations are accurate
3. Review Redis key expiration settings

## Security Considerations

- **User Isolation**: Each user has separate spending tracking
- **Tamper Proof**: Spending data stored server-side in Redis
- **No Client Override**: Limits enforced server-side only
- **Admin Only Reset**: Only admins can reset spending via CLI

## Future Enhancements

Potential improvements:

- Per-user custom limits (override global configuration)
- Email/webhook notifications when approaching limits
- Spending analytics and reporting
- Grace period before hard blocking
- Automatic limit adjustment based on historical usage
- Integration with billing/payment systems for automatic top-ups
