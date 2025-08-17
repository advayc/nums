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

### Persistent Storage (Redis / Upstash)
To avoid counters resetting you must use a Redis service with persistence. The free Redis Cloud plan (no persistence) will wipe data periodically. Upstash free tier persists by default.

#### Upstash Setup
1. Create a free Redis database at https://console.upstash.com/ (choose a region near your users).
2. Copy the generated credentials. You will see either:
	- A full URL: `rediss://default:<PASSWORD>@<HOST>:<PORT>` (use this as `REDIS_URL`), or
	- Separate host + password (set `UPSTASH_REDIS_URL` and `UPSTASH_REDIS_PASSWORD`). The server will auto-build `REDIS_URL` if `REDIS_URL` itself is empty.
3. In your `.env` (or Vercel project settings) set:
	- `SECRET_TOKEN=<shared secret>`
	- `REDIS_URL=rediss://default:<PASSWORD>@<HOST>:<PORT>` (or the two Upstash vars)
	- `ALLOWED_ORIGINS=https://your-site.vercel.app`
	- `FAIL_FAST_REDIS=1` (recommended so deploy fails if Redis is unreachable)
	- Frontend (Next.js) also needs `HIT_COUNTER_SECRET_TOKEN` (same value) and `NEXT_PUBLIC_HIT_COUNTER_URL` pointing at the deployed hit service.
4. Redeploy. Logs should show: `redis persistence enabled ...`.
5. Test:
```bash
curl -H "X-Auth-Token: $SECRET_TOKEN" "https://<hit-service-domain>/hit?id=home"
curl -H "X-Auth-Token: $SECRET_TOKEN" "https://<hit-service-domain>/count?id=home&format=txt"
```

#### Security Notes
Never commit real secrets. Use the provided `.env.example` as a template. Rotate credentials immediately if exposed.

#### Fail-Fast Behavior
If you set `FAIL_FAST_REDIS=1`, startup aborts instead of silently falling back to in-memory counters when Redis init fails, preventing accidental resets.

## Notes
- Ensure the `SECRET_TOKEN` matches between the backend and frontend.
- Logs are available in the Next.js API route for debugging upstream interactions.
 - Use Upstash (or a persistent Redis plan) to guarantee counters survive restarts.
  
<img width="1920" height="1093" alt="banner" src="https://github.com/user-attachments/assets/bd074a80-ea82-43a6-9649-bc00ab7d1446" />
