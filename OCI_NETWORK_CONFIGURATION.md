# OCI Network Configuration Required

**Status**: Security list updated, but connectivity still failing
**Date**: 2026-01-13

## Problem Summary

The jumphost (132.145.179.230 / 10.0.1.71) cannot reach the production server (129.80.158.147) even after adding VCN-wide SSH rules.

## What Was Done

### 1. Security List Updated
- **Security List**: `ocid1.securitylist.oc1.iad.aaaaaaaav5owplb7jdsantmoijvjb7ybofe4wd734h4gelbrsp5dpksmnooq`
- **Subnet**: `darkapi-public-subnet` (10.0.1.0/24)
- **Rule Added**: Allow SSH (port 22) from 10.0.0.0/16 (entire VCN)

### 2. Test Result
```bash
$ ssh -i ~/.ssh/darkapi_key opc@132.145.179.230 "nc -z 129.80.158.147 22"
✗ Connection failed (100% packet loss)
```

## Possible Causes

### 1. Instances in Different Subnets
- **Jumphost**: Confirmed in 10.0.1.0/24 (darkapi-public-subnet)
  - Private IP: 10.0.1.71
  - Public IP: 132.145.179.230
- **Production Server**: Unknown subnet
  - Public IP: 129.80.158.147
  - Private IP: Unknown

### 2. Production Server Has Different Security List
If the production server is in a different subnet, it would have its own security list that may not allow incoming SSH from the VCN.

### 3. Network Security Group (NSG) Rules
The production server may have NSG rules that override security list rules.

### 4. Different VCN
The production server might be in a completely different VCN.

## Next Steps to Resolve

### Option 1: Find Production Server Subnet (Recommended)

**Via OCI Console**:
1. Go to: https://cloud.oracle.com
2. Navigate to: Compute → Instances
3. Search for IP: `129.80.158.147` or hostname: `aiserve.farm`
4. Click on the instance
5. Note:
   - Subnet ID
   - Security Lists attached
   - Any Network Security Groups (NSGs)
   - Private IP address

**Via OCI CLI** (if you can find the instance OCID):
```bash
# Find instance OCID first
oci compute instance list --all \
  --query "data[?\"display-name\" contains '` aiserve'` || \"display-name\" contains 'models'].{name:\"display-name\",id:id,ip:\"public-ip\"}"

# Then get full details
oci compute instance get --instance-id <OCID> \
  --query 'data.{name:"display-name",subnet:"subnet-id",private_ip:"primary-private-ip"}'
```

### Option 2: Add Security Rules to Production Server Subnet

Once you find the subnet, add this ingress rule to its security list:

```bash
# Get security list for production server's subnet
oci network subnet get --subnet-id <PROD_SUBNET_ID> \
  --query 'data."security-list-ids"'

# Update that security list
oci network security-list update \
  --security-list-id <PROD_SECURITY_LIST_ID> \
  --ingress-security-rules <ADD_SSH_RULE_FROM_VCN>
```

### Option 3: Check for Network Security Groups

```bash
# Check if instance has NSGs
oci compute instance get --instance-id <PROD_INSTANCE_ID> \
  --query 'data.vnic-attachments[].{"nsg-ids":"nsg-ids"}'

# If NSGs exist, add rule to allow SSH from VCN
oci network nsg rules add --nsg-id <NSG_ID> \
  --security-rules '[{
    "direction": "INGRESS",
    "protocol": "6",
    "source": "10.0.0.0/16",
    "source-type": "CIDR_BLOCK",
    "tcp-options": {
      "destination-port-range": {"min": 22, "max": 22}
    },
    "description": "Allow SSH from VCN for management"
  }]'
```

### Option 4: Use Cloudflare Tunnel (Alternative)

If network rules can't be modified immediately, use Cloudflare Tunnel as documented in `/Users/ryan/development/oci-cloudflared/`:

**Advantages**:
- Bypasses all OCI network rules
- Already configured for Cloudflare IP ranges (visible in security list)
- Secure access via Cloudflare Access

**Steps**:
1. Ensure Cloudflare tunnel is running on production server
2. Add DNS record for aiserve.farm to tunnel
3. Use `cloudflared access ssh` for deployment

## Current Security List Rules

The `darkapi-public-subnet` security list allows:
- SSH (22) from user's home IP: `24.46.203.96/32`
- SSH (22) from Cloudflare IP ranges (many entries)
- SSH (22) from VCN: `10.0.0.0/16` ✅ (NEWLY ADDED)
- HTTPS (443) from anywhere: `0.0.0.0/0`
- HTTP (80) from anywhere: `0.0.0.0/0`
- Port 8081 from VCN: `10.0.0.0/16`

## Subnets in the Tenancy

```
darkapi-public-subnet      10.0.1.0/24  (jumphost is here)
darkapi-private-subnet     10.0.2.0/24
darkapi-database-subnet    10.0.3.0/24
oke-worker-subnet          10.0.1.0/24  (different VCN)
oke-lb-subnet              10.0.2.0/24  (OKE VCN)
oke-api-endpoint-subnet    10.0.0.0/24  (OKE VCN)
managedcrypto-subnet       10.0.10.0/24
ads-build-subnet           10.0.1.0/24  (different VCN)
webscience-public-subnet   10.20.1.0/24 (different VCN)
hostscience-public-subnet  10.10.1.0/24 (different VCN)
diseasezone-public-subnet  10.0.1.0/24  (different VCN)
```

**Note**: Multiple subnets use `10.0.1.0/24` but are in different VCNs!

## Deployment Strategy Once Network Fixed

Once SSH access is working:

```bash
# Test connectivity
ssh -i ~/.ssh/darkapi_key opc@132.145.179.230 "nc -z 129.80.158.147 22"

# Deploy website
/Users/ryan/development/adsops-utils/tools/aiserve-deploy
```

## Quick Check Commands

```bash
# From jumphost, check which IPs are reachable
ssh -i ~/.ssh/darkapi_key opc@132.145.179.230 "
  for ip in {1..254}; do
    timeout 0.5 nc -z 10.0.1.\$ip 22 2>/dev/null && echo \"10.0.1.\$ip:22 open\"
  done
"

# Check if production server is even listening on port 22
nmap -p 22 129.80.158.147

# Check from user's local machine (should work based on existing rules)
ssh -i ~/.ssh/darkapi_key opc@129.80.158.147 "hostname"
```

## Files Created Today

1. `/usr/local/sshkeys/get-tunnel-helper` - Tunnel utility finder
2. `/usr/local/sshkeys/.env` - SSH key management config
3. `/usr/local/bin/n8ntunnel.py` - OCI bastion tunnel helper
4. `/usr/local/bin/oci-tunnel.py` - OCI tunnel utility
5. `/Users/ryan/development/adsops-utils/tools/aiserve-deploy` - Deployment script
6. `/Users/ryan/development/aiserve-gpuproxyd/DEPLOYMENT_STATUS.md` - Full status doc
7. PostgreSQL `inventory` database with aiserve.farm entry

## Architecture Clarifications

From user:
1. Jumphost → All servers EXCEPT Keycloak ✅
2. SSH keys → Stored in Vault
3. Keycloak → Only via Cloudflare tunnel ✅

---

**Action Required**: Identify which subnet the production server (129.80.158.147) is in and ensure its security list allows SSH from the VCN or specifically from the jumphost (10.0.1.71).
