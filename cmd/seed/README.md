# Database Seed Utility

Bootstrap test users and API keys for local development.

## Configuration

The seed utility reads from `.local_admin` (configurable) which contains admin and client user credentials.

### .local_admin Format

```ini
# Local Admin Configuration - Test Keys Only
# XXX: CHANGEME - These are pre-seeded test keys for development only
# NEVER use these keys in production!

[admins]
ryan=test_password_ryan123
david=test_password_david456

[clients]
client1=test_password_client1
client2=test_password_client2
client3=test_password_client3
```

## Usage

### Prerequisites

1. Ensure PostgreSQL and Redis are running
2. Set up your `.env` file with database credentials
3. Run migrations first: `./admin migrate`

### Dry Run (Preview Changes)

```bash
./bin/seed --dry-run
```

This will show what users would be created without actually creating them.

### Run Seeding

```bash
./bin/seed
```

This will:
1. Create admin users (ryan@admin.local, david@admin.local) with admin privileges
2. Create client users (client1@client.local, client2@client.local, client3@client.local)
3. Generate a default API key for each user
4. Display the generated API keys (save these!)

### Custom Config File

```bash
./bin/seed --config /path/to/custom/config
```

## Generated Credentials

After seeding, you'll have:

**Admin Users:**
- ryan@admin.local (password: test_password_ryan123)
- david@admin.local (password: test_password_david456)

**Client Users:**
- client1@client.local (password: test_password_client1)
- client2@client.local (password: test_password_client2)
- client3@client.local (password: test_password_client3)

Each user will have a default API key generated (displayed during seeding).

## Security Notes

⚠️ **IMPORTANT:** These credentials are for **LOCAL DEVELOPMENT ONLY**

- All emails use `.local` TLD to prevent accidental use in production
- Passwords are simple and marked with `XXX: CHANGEME` comments
- API keys are displayed once during seeding - save them securely
- Never commit `.local_admin` with real credentials to version control

## Building

```bash
go build -o bin/seed ./cmd/seed
```

## Related Commands

After seeding, you can use the admin utility for additional operations:

```bash
# List all users
./admin users

# Create additional API keys
./admin create-apikey ryan@admin.local ryan-secondary-key

# Show user statistics
./admin stats
```
