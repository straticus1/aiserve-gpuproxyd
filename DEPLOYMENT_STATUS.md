# AIServe.Farm Deployment Status

**Date**: 2026-01-13
**Status**: Ready for OCI Network Configuration

## What's Been Completed

### 1. Infrastructure Inventory System ✅
- **Database**: PostgreSQL `inventory` database created at `localhost:5432`
- **Schema**: Added `access_method` fields to track SSH, jumphost, tunnel, bastion access
- **Server Entry**: `aiserve.farm-web` added to inventory:
  - Hostname: `129.80.158.147`
  - Jumphost: `132.145.179.230`
  - Access methods: `jumphost`, `ssh`
  - SSH user: `opc`
  - SSH key: `~/.ssh/darkapi_key`

**Database Table**:
```sql
SELECT resource_name, hostname, access_methods, ssh_user, access_config
FROM inventory_resources
WHERE hostname = '129.80.158.147';

 resource_name    | hostname       | access_methods | ssh_user | access_config
------------------+----------------+----------------+----------+----------------------------
 aiserve.farm-web | 129.80.158.147 | {jumphost,ssh} | opc      | {"jumphost": "132.145.179.230", ...}
```

### 2. Deployment Tools Created ✅

**Location**: `/Users/ryan/development/adsops-utils/tools/`

1. **get-tunnel-helper** - Displays available tunnel utilities
   - Lists n8ntunnel.py, oci-tunnel.py, cloudflared
   - Shows Cloudflare tunnel configuration
   - Usage examples for each method

2. **aiserve-deploy** - Automated deployment script
   - Packages website files
   - Checks server accessibility (direct, jumphost, tunnel)
   - Uploads and deploys files
   - Restarts services
   - Verifies deployment

3. **hostctl** (existing) - Host inventory management CLI
   - Source code directory with Go implementation
   - Binary compiled for macOS and Linux
   - Needs `INVENTORY_DB_PASSWORD` environment variable

### 3. Website Files Ready ✅

**Source**: `/Users/ryan/development/aiserve-gpuproxyd/web/`
- `index.html` (17KB) - New AIServe.Farm landing page
- `styles.css` (11KB) - Dark theme styling
- `app.js` (13KB) - Keycloak authentication integration

**Features**:
- GPU marketplace with "Rent Now" buttons
- Pricing tiers (Starter/Pro/Enterprise)
- Login/Signup modals
- Keycloak OAuth2 integration
- Responsive mobile design

### 4. Tunnel Utilities Deployed ✅

**Location**: `/usr/local/bin/`
- `n8ntunnel.py` - OCI Bastion tunnel helper
- `oci-tunnel.py` - OCI tunnel utility

**Configuration Files**:
- `/usr/local/sshkeys/.env` - SSH key management configuration
- `/Users/ryan/development/oci-cloudflared/config.yml` - Cloudflare tunnel config

## Current Blocker: OCI Network Rules

### Problem
The jumphost (132.145.179.230) **CANNOT** reach the production server (129.80.158.147):
```bash
$ ssh -i ~/.ssh/darkapi_key opc@132.145.179.230 "ping -c 2 129.80.158.147"
--- 129.80.158.147 ping statistics ---
2 packets transmitted, 0 received, 100% packet loss
```

### Root Cause
OCI Security List rules do not allow SSH (port 22) from the jumphost to the production server.

### Architecture Requirements (from user)
1. **Jumphost** → All servers EXCEPT Keycloak
2. **SSH keys** → Stored in Vault
3. **Keycloak** → Only via Cloudflare tunnel

## What Needs to Be Done

### Step 1: Configure OCI Security Lists

**Goal**: Allow SSH (port 22) from jumphost to production server

**Option A**: Via OCI Console
1. Go to: https://cloud.oracle.com
2. Navigate to: Networking → Virtual Cloud Networks
3. Find the VCN containing instance `129.80.158.147`
4. Click on Security Lists
5. Find the security list for the production server's subnet
6. Add Ingress Rule:
   - **Source CIDR**: `132.145.179.230/32`
   - **IP Protocol**: TCP
   - **Destination Port**: 22
   - **Description**: "Allow SSH from jumphost for management"

**Option B**: Via OCI CLI
```bash
# Find the instance and subnet
oci compute instance list --all \
  --query "data[?\"public-ip\" == '129.80.158.147'].{id:id, subnet:\"subnet-id\"}"

# Get security list ID
oci network subnet get --subnet-id <SUBNET_ID> \
  --query 'data."security-list-ids"'

# Update security list (add jumphost rule)
oci network security-list update \
  --security-list-id <SECURITY_LIST_ID> \
  --ingress-security-rules '[{
    "source": "132.145.179.230/32",
    "protocol": "6",
    "tcp-options": {"destination-port-range": {"min": 22, "max": 22}},
    "description": "Allow SSH from jumphost"
  }]' \
  --force
```

### Step 2: Verify Connectivity
```bash
# Test from jumphost to production
ssh -i ~/.ssh/darkapi_key opc@132.145.179.230 \
  "timeout 5 nc -z 129.80.158.147 22 && echo 'Connection successful!'"
```

### Step 3: Deploy Website
Once connectivity is established:
```bash
cd /Users/ryan/development/adsops-utils/tools
./aiserve-deploy
```

This will:
1. Package web files
2. Upload via jumphost
3. Deploy to `/home/opc/aiserve-gpuproxyd/web`
4. Restart service/container
5. Verify at https://aiserve.farm

## Alternative: Cloudflare Tunnel Deployment

If OCI network rules can't be modified immediately, use Cloudflare Tunnel:

**Prerequisites**:
- Cloudflare tunnel must be running on the production server
- DNS must point to tunnel (not currently configured for aiserve.farm)

**Steps**:
1. Configure tunnel ingress for aiserve.farm
2. Use `cloudflared access ssh` for deployment
3. Update DNS to use tunnel

## Quick Reference

**Jumphost Access**:
```bash
ssh -i ~/.ssh/darkapi_key opc@132.145.179.230
```

**Production Server** (after network rules fixed):
```bash
ssh -i ~/.ssh/darkapi_key -J opc@132.145.179.230 opc@129.80.158.147
```

**Check Deployment**:
```bash
curl -I https://aiserve.farm
```

**Inventory Server**:
```bash
# Running on localhost:3456
curl http://localhost:3456/health
```

## Files Modified/Created

### New Files
- `/usr/local/sshkeys/.env` - SSH key management config
- `/usr/local/sshkeys/get-tunnel-helper` - Tunnel utility finder
- `/usr/local/bin/n8ntunnel.py` - OCI bastion tunnel helper
- `/usr/local/bin/oci-tunnel.py` - OCI tunnel utility
- `/Users/ryan/development/adsops-utils/tools/aiserve-deploy` - Deployment script
- `/Users/ryan/development/adsops-utils/tools/.env` - Copied config
- `/Users/ryan/development/adsops-utils/tools/get-tunnel-helper` - Copied tool

### Database Changes
```sql
-- New enum type
CREATE TYPE access_method_type AS ENUM (
  'ssh', 'tunnel', 'bastion', 'cloudflare_tunnel', 'oci_bastion', 'jumphost', 'direct'
);

-- New columns in inventory_resources
ALTER TABLE inventory_resources ADD COLUMN access_methods access_method_type[];
ALTER TABLE inventory_resources ADD COLUMN access_config JSONB;
ALTER TABLE inventory_resources ADD COLUMN ssh_key_path VARCHAR(500);
ALTER TABLE inventory_resources ADD COLUMN ssh_user VARCHAR(100);
ALTER TABLE inventory_resources ADD COLUMN tunnel_config JSONB;
```

## Next Session

When you return to this:
1. Fix OCI network rules to allow jumphost → production server
2. Run: `/Users/ryan/development/adsops-utils/tools/aiserve-deploy`
3. Verify: https://aiserve.farm shows new site
4. Update DNS for api.aiserve.farm (currently no DNS record)

## Notes

- Production server `129.80.158.147` is confirmed to be an Oracle Cloud instance
- Server is reachable via HTTPS (443) but not responding on port 8080 externally
- Jumphost `132.145.179.230` is accessible and responding
- Database `inventory` is initialized and running
- Inventory server is running on port 3456
- Git commit ready: `cd2c67d` (web files committed to GitHub)

---

**Status**: Waiting for OCI Security List configuration to enable jumphost access.

**Contact**: ryan@afterdarksys.com
