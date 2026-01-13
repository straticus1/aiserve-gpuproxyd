# Changelog

All notable changes to AIServe.Farm will be documented in this file.

**AIServe.Farm** - GPU Proxy, AI Model, Inference, MCP, and Agent Platform by AfterDark Systems (ADS)

## [1.0.1] - 2026-01-13

### Security Fixes
- **CRITICAL**: Removed exposed API keys from `.env.production` file
- Added comprehensive `.gitignore` patterns to prevent credential exposure
- Enhanced security warnings in configuration files

### Bug Fixes
- **CRITICAL**: Fixed ONNX Runtime double-free vulnerability that caused segmentation faults
- **CRITICAL**: Fixed model loading goroutine memory leaks with proper panic recovery and context timeouts
- Fixed Redis spending race condition in guardrails middleware

### Architecture Changes
- **BREAKING**: Replaced AWS S3 SDK with OCI Object Storage SDK
  - All storage operations now use Oracle Cloud Infrastructure
  - Updated configuration to use OCI credentials and endpoints
  - Maintained S3-compatible interface for backward compatibility

### New Features
- Added comprehensive storage quota management system
  - Per-user storage limits (default: 100GB, premium: 1TB)
  - File size limits (default: 10GB, premium: 100GB)
  - Hourly upload rate limiting (default: 50/hour, premium: 500/hour)
  - Daily upload rate limiting (default: 500/day, premium: 5000/day)
- Added `/api/v1/quota` endpoint to check current usage and limits
- Implemented automatic cleanup of expired rate limit timestamps

### Performance Improvements
- Enhanced goroutine lifecycle management with proper context cancellation
- Added 5-minute timeout for model loading operations
- Improved error handling and recovery in background tasks

### Configuration
- Added OCI Object Storage configuration options:
  - `OCI_STORAGE_ENDPOINT`
  - `OCI_STORAGE_NAMESPACE`
  - `OCI_STORAGE_BUCKET`
  - `OCI_ACCESS_KEY_ID`
  - `OCI_SECRET_ACCESS_KEY`

### Documentation
- Updated README with OCI storage information
- Added MIT LICENSE file
- Enhanced environment variable documentation
- Created CHANGELOG for version tracking

### Dependencies
- Added `github.com/oracle/oci-go-sdk/v65` for OCI Object Storage
- Removed AWS SDK dependencies

## [1.0.0] - 2026-01-12

### Initial Release
- Multi-provider GPU access (vast.ai, io.net)
- ML Runtime support (ONNX, PyTorch, TensorFlow, scikit-learn)
- Load balancing with 5 strategies
- GPU reservation system
- Multiple protocol support (HTTP/HTTPS, gRPC, MCP, WebSocket)
- Authentication with JWT tokens and API keys
- Payment integration (Stripe, Crypto, AfterDark)
- Guard rails for spending control
- Agent protocol support (MCP, A2A, ACP, FIPA ACL, KQML, LangChain)
- Comprehensive CLI tools
- Database support (PostgreSQL, SQLite)
- Redis session management
- Syslog and file logging
