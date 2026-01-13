# Keycloak Setup Guide for AIServe.Farm

## Overview

AIServe.Farm uses Keycloak (After Dark Systems Central Login) for centralized authentication and authorization.

## Prerequisites

- Keycloak server running at `https://auth.afterdarksys.com`
- Admin access to Keycloak
- Keycloak CLI tools installed

## Installation

### 1. Install Keycloak (if not already running)

```bash
# Download Keycloak
wget https://github.com/keycloak/keycloak/releases/download/23.0.0/keycloak-23.0.0.tar.gz
tar -xzf keycloak-23.0.0.tar.gz
cd keycloak-23.0.0

# Start Keycloak
bin/kc.sh start-dev
```

### 2. Access Keycloak Admin Console

1. Navigate to `https://auth.afterdarksys.com/admin`
2. Login with admin credentials
3. Select the `afterdark` realm (or create it)

## Realm Configuration

### Create Realm: `afterdark`

```bash
# Using Keycloak CLI
kcadm.sh create realms -s realm=afterdark -s enabled=true
```

Or via Admin Console:
1. Click "Add Realm"
2. Name: `afterdark`
3. Enabled: ON
4. Save

## Client Configuration

### Create Client: `aiserve-farm`

#### Via CLI:

```bash
kcadm.sh create clients -r afterdark \
  -s clientId=aiserve-farm \
  -s enabled=true \
  -s publicClient=true \
  -s directAccessGrantsEnabled=true \
  -s 'redirectUris=["https://aiserve.farm/*","http://localhost:*/*"]' \
  -s 'webOrigins=["https://aiserve.farm","http://localhost"]'
```

#### Via Admin Console:

1. Go to **Clients** → **Create**
2. **Client ID**: `aiserve-farm`
3. **Client Protocol**: openid-connect
4. Save

#### Client Settings:

**Settings Tab:**
- **Access Type**: public
- **Standard Flow Enabled**: ON
- **Direct Access Grants Enabled**: ON
- **Valid Redirect URIs**:
  - `https://aiserve.farm/*`
  - `http://localhost:*/*`
- **Web Origins**:
  - `https://aiserve.farm`
  - `http://localhost`
- **Base URL**: `https://aiserve.farm`

**Advanced Settings:**
- **Access Token Lifespan**: 5 minutes
- **SSO Session Idle**: 30 minutes
- **SSO Session Max**: 10 hours

Save settings.

## User Federation

### Link to After Dark Systems Central

If you want to federate with an existing user database:

1. Go to **User Federation** → **Add provider**
2. Select provider type (LDAP, AD, etc.)
3. Configure connection settings
4. Test connection
5. Sync users

## Client Scopes

### Create Custom Scopes for AIServe.Farm

```bash
# Create scope
kcadm.sh create client-scopes -r afterdark \
  -s name=aiserve \
  -s protocol=openid-connect
```

#### Add mappers:

```bash
# User ID mapper
kcadm.sh create client-scopes/<scope-id>/protocol-mappers/models -r afterdark \
  -s name=user-id \
  -s protocol=openid-connect \
  -s protocolMapper=oidc-usermodel-property-mapper \
  -s 'config."user.attribute"=id' \
  -s 'config."claim.name"=sub' \
  -s 'config."id.token.claim"=true' \
  -s 'config."access.token.claim"=true'

# Email mapper
kcadm.sh create client-scopes/<scope-id>/protocol-mappers/models -r afterdark \
  -s name=email \
  -s protocol=openid-connect \
  -s protocolMapper=oidc-usermodel-property-mapper \
  -s 'config."user.attribute"=email' \
  -s 'config."claim.name"=email' \
  -s 'config."id.token.claim"=true' \
  -s 'config."access.token.claim"=true'
```

## Roles

### Create AIServe.Farm Roles

```bash
# Create realm roles
kcadm.sh create roles -r afterdark -s name=aiserve-user
kcadm.sh create roles -r afterdark -s name=aiserve-pro
kcadm.sh create roles -r afterdark -s name=aiserve-enterprise
kcadm.sh create roles -r afterdark -s name=aiserve-admin
```

#### Via Admin Console:

1. Go to **Roles** → **Add Role**
2. Create roles:
   - `aiserve-user` - Free tier
   - `aiserve-pro` - Pro tier
   - `aiserve-enterprise` - Enterprise tier
   - `aiserve-admin` - Admin access

## Environment Variables

Add these to your `.env` file:

```bash
# Keycloak Configuration
KEYCLOAK_URL=https://auth.afterdarksys.com
KEYCLOAK_REALM=afterdark
KEYCLOAK_CLIENT_ID=aiserve-farm
KEYCLOAK_CLIENT_SECRET=<your-client-secret>

# Optional: Keycloak Admin API
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=<admin-password>
```

## Backend Integration

### Install Keycloak Go Adapter

```bash
go get github.com/Nerzal/gocloak/v13
```

### Example Integration Code

```go
package main

import (
    "context"
    "github.com/Nerzal/gocloak/v13"
)

func initKeycloak() (*gocloak.GoCloak, error) {
    client := gocloak.NewClient("https://auth.afterdarksys.com")

    // Admin login
    token, err := client.LoginAdmin(context.Background(),
        "admin",
        "password",
        "master",
    )
    if err != nil {
        return nil, err
    }

    return client, nil
}

func verifyToken(token string) (*gocloak.JWT, error) {
    client := gocloak.NewClient("https://auth.afterdarksys.com")

    result, err := client.RetrospectToken(context.Background(),
        token,
        "aiserve-farm",
        "client-secret",
        "afterdark",
    )

    if err != nil {
        return nil, err
    }

    if !*result.Active {
        return nil, fmt.Errorf("token not active")
    }

    return result, nil
}
```

## Testing

### Test Authentication Flow

```bash
# Get authorization URL
curl "https://auth.afterdarksys.com/realms/afterdark/protocol/openid-connect/auth?\
client_id=aiserve-farm&\
redirect_uri=http://localhost:8080&\
response_type=code&\
scope=openid"

# Exchange code for token
curl -X POST "https://auth.afterdarksys.com/realms/afterdark/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code" \
  -d "client_id=aiserve-farm" \
  -d "code=<authorization-code>" \
  -d "redirect_uri=http://localhost:8080"
```

### Test with cURL

```bash
# Login and get token
TOKEN=$(curl -X POST "https://auth.afterdarksys.com/realms/afterdark/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=aiserve-farm" \
  -d "username=testuser" \
  -d "password=testpass" \
  -d "grant_type=password" | jq -r '.access_token')

# Use token
curl -H "Authorization: Bearer $TOKEN" https://api.aiserve.farm/api/v1/gpu/instances
```

## Security Best Practices

### 1. Enable HTTPS

Keycloak requires HTTPS in production:

```bash
# Generate SSL certificate
openssl req -x509 -newkey rsa:4096 \
  -keyout key.pem -out cert.pem \
  -days 365 -nodes

# Configure Keycloak with cert
bin/kc.sh start \
  --https-certificate-file=cert.pem \
  --https-certificate-key-file=key.pem
```

### 2. Configure CORS

In Keycloak Admin Console:
1. Go to **Realm Settings** → **Security Defenses**
2. **CORS**: Add `https://aiserve.farm`
3. Save

### 3. Enable Rate Limiting

```bash
# Install Keycloak rate limiting extension
# Add to standalone.xml or use reverse proxy (nginx/cloudflare)
```

### 4. Setup MFA (Optional)

1. Go to **Authentication** → **Flows**
2. Copy "Browser" flow
3. Add **OTP Form** step
4. Set as required
5. Bind to browser flow

## User Management

### Create Test User

```bash
# Via CLI
kcadm.sh create users -r afterdark \
  -s username=testuser \
  -s email=test@example.com \
  -s enabled=true

# Set password
kcadm.sh set-password -r afterdark \
  --username testuser \
  --new-password testpass
```

### Assign Role

```bash
kcadm.sh add-roles -r afterdark \
  --uusername testuser \
  --rolename aiserve-user
```

## Monitoring

### Enable Keycloak Events

1. Go to **Realm Settings** → **Events**
2. **Event Listeners**: Add `jboss-logging`
3. **Save Events**: ON
4. **Saved Types**: Select all login/logout events
5. **Expiration**: 30 days

### View Events

1. Go to **Events** → **Login Events**
2. Filter by user, client, or date
3. Export for analysis

## Troubleshooting

### Issue: "Invalid redirect_uri"

**Solution**: Add redirect URI to client settings:
```
https://aiserve.farm/*
http://localhost:*/*
```

### Issue: "CORS error"

**Solution**: Add web origin in client settings:
```
https://aiserve.farm
```

### Issue: "Token expired"

**Solution**: Increase token lifespan in client settings or implement refresh token flow.

### Issue: "User not found"

**Solution**: Check user federation sync or create user manually.

## Production Checklist

- [ ] HTTPS enabled
- [ ] SSL certificates installed
- [ ] CORS configured
- [ ] Redirect URIs whitelisted
- [ ] Rate limiting enabled
- [ ] Event logging enabled
- [ ] Backup strategy in place
- [ ] MFA configured (optional)
- [ ] Admin password changed
- [ ] Database backed up regularly

## Resources

- **Keycloak Documentation**: https://www.keycloak.org/documentation
- **Admin CLI**: https://www.keycloak.org/docs/latest/server_admin/#the-admin-cli
- **Go Adapter**: https://github.com/Nerzal/gocloak
- **After Dark Systems Support**: support@afterdarksys.com

## Support

For Keycloak issues:
- Email: support@afterdarksys.com
- Internal wiki: https://wiki.afterdarksys.com/keycloak
- Slack: #auth-support
