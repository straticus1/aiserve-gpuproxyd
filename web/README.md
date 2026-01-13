# AIServe.Farm Frontend

Complete frontend implementation for aiserve.farm GPU marketplace.

## Files

- **`index.html`** - Main landing page with all sections
- **`styles.css`** - Complete styling system (dark theme)
- **`app.js`** - JavaScript application logic and API integration
- **`admin/`** - Admin dashboard (existing)

## Quick Start

### Local Development

1. Start the Go server:
```bash
cd /Users/ryan/development/aiserve-gpuproxyd
go run cmd/server/main.go
```

2. Open browser:
```
http://localhost:8080
```

### Features

- ✅ Landing page with hero section
- ✅ GPU marketplace with rent buttons
- ✅ Pricing tiers (Starter/Pro/Enterprise)
- ✅ Login/Signup modals
- ✅ Keycloak integration
- ✅ Responsive design
- ✅ API integration

## Configuration

Edit `app.js` to change API endpoints:

```javascript
const CONFIG = {
    API_URL: 'https://api.aiserve.farm/api/v1',
    KEYCLOAK_URL: 'https://auth.afterdarksys.com',
    KEYCLOAK_REALM: 'afterdark',
    KEYCLOAK_CLIENT_ID: 'aiserve-farm'
};
```

## Deployment

### Option 1: Nginx

```nginx
server {
    listen 80;
    server_name aiserve.farm;
    root /path/to/web;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://localhost:8080;
    }
}
```

### Option 2: Go Server

The Go server automatically serves files from `./web`:

```go
router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))
```

## Testing

Open `index.html` directly in browser or use Python server:

```bash
cd web
python3 -m http.server 8000
```

Then visit: http://localhost:8000

## Next Steps

1. Setup Keycloak (see `docs/KEYCLOAK_SETUP.md`)
2. Configure DNS for aiserve.farm
3. Setup SSL certificates
4. Deploy to production

## Documentation

See `WEB_OVERHAUL_COMPLETE.md` for complete implementation details.
