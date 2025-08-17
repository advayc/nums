# nums

This project is a hit counter service built with a Go backend and a Next.js API proxy. It allows you to track and display page views for your website.

## Features
- Increment and fetch hit counts for specific IDs.
- Proxy API route in Next.js to securely interact with the backend.
- Supports plain-text counts and SVG badges.

## Usage

### API Endpoints
- **Increment Hits**: `POST /api/hit?id=<id>`
- **Fetch Hits**: `GET /api/hit?id=<id>`

### Example
Increment the hit count for `test`:
```bash
curl -i -X POST "http://localhost:3000/api/hit?id=test"
```

Fetch the hit count for `test`:
```bash
curl -i "http://localhost:3000/api/hit?id=test"
```

## Deployment
- Deploy the Go server to a platform like Vercel.
- Deploy the Next.js app to Vercel or any other hosting provider.

## Notes
- Ensure the `SECRET_TOKEN` matches between the backend and frontend.
- Logs are available in the Next.js API route for debugging upstream interactions.
  
<img width="1920" height="1093" alt="banner" src="https://github.com/user-attachments/assets/bd074a80-ea82-43a6-9649-bc00ab7d1446" />
