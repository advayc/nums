# nums

This project is a hit counter service built with a Go backend and a Next.js API proxy. It allows you to track and display page views for your website.

<img width="1920" height="1093" alt="banner" src="https://github.com/user-attachments/assets/bd074a80-ea82-43a6-9649-bc00ab7d1446" />

## What it gives you
- `/hit?id=foo` (GET/POST): increments and returns JSON `{ id, hits }`
- `/count?id=foo`: returns current JSON count
- `/count.txt?id=foo`: plain number (good for badges)
- `/badge?id=foo&label=views`: SVG badge (no increment)
- Auth (optional) via `SECRET_TOKEN` header `X-Auth-Token` (or `?token=`)
- Persistent storage in Upstash Redis (falls back to in‑memory if unavailable)

---
## Quick Setup (Fork -> Upstash -> Run)

### 1. Fork & Clone
```bash
git clone https://github.com/advayc/nums.git
cd nums
```

### 2. Create Upstash Redis
1. Go to https://console.upstash.com/redis and create a database.
2. Copy the Endpoint (host:port) and Password.

### 3. Create `.env` (no `.env.example` used)
Paste & fill (leave `REDIS_URL` blank to auto-build from `UPSTASH_` values):
```
PORT=8080
SECRET_TOKEN=REPLACE_WITH_RANDOM_SECRET
PERSIST_FILE=/tmp/counter.txt
ALLOWED_ORIGINS=YOURWEBSITE
REDIS_URL=
UPSTASH_REDIS_URL=
UPSTASH_REDIS_PASSWORD=
REDIS_PREFIX=hits:
FAIL_FAST_REDIS=0
HIT_COUNTER_SECRET_TOKEN=REPLACE_WITH_RANDOM_SECRET
NEXT_PUBLIC_HIT_COUNTER_URL=YOURVERCELDEPLOYMENTURL
```
Minimum for persistence: `SECRET_TOKEN` plus either `REDIS_URL` or both `UPSTASH_REDIS_URL` & `UPSTASH_REDIS_PASSWORD`.

### 4. Run Locally
```bash
go run ./cmd/server
curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:8080/hit?id=home"
curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:8080/count?id=home"
```
Expect `{ "id": "home", "hits": 1 }`.

### 5. Deploy to Vercel
1. Import the repo into Vercel.
2. Add all env vars (omit `PORT` / `PERSIST_FILE` if you want—they're ignored serverless).
3. Deploy → base URL: `https://<deployment>`.

### 6. Use in Next.js (simple client component)
```tsx
"use client";
import { useEffect, useState } from 'react';
export function HitCounter({ id }: { id: string }) {
  const [hits, setHits] = useState<number>();
  useEffect(() => {
    fetch(`${process.env.NEXT_PUBLIC_HIT_COUNTER_URL}/hit?id=${encodeURIComponent(id)}`, {
      headers: { 'X-Auth-Token': process.env.HIT_COUNTER_SECRET_TOKEN! }
    }).then(r => r.json()).then(d => setHits(d.hits));
  }, [id]);
  return <span>{hits ?? '…'}</span>;
}
```

### 7. Badges / Plain Count
Public read endpoints (`/count`, `/count.txt`, `/badge`, `/badge.json`) do NOT require the auth token; only `/hit` does.

#### Recommended for GitHub (using shields.io)
```markdown
![hits](https://img.shields.io/endpoint?url=https%3A%2F%2Fnums-ten.vercel.app%2Fbadge.json%3Fid%3Dhome%26label%3Dhits%26cacheSeconds%3D30)
```
- **Label**: Change `label=hits` to any text you want (e.g., `views`).
- **Update interval**: About every 30 seconds (minimum allowed by Shields.io).
- **Works everywhere**: Always renders on GitHub and most platforms.

#### Direct terminal-style badge (custom SVG, updates instantly, through vercel deployment)
```markdown
![hits](https://nums-ten.vercel.app/badge?id=home&style=terminal&label=hits)
```

- **Label**: Set `label=hits` or any text.
- **Update interval**: Instantly from your server, but GitHub may cache for 1–2 minutes.
- **Customizable**: Change colors and font with `bg`, `labelColor`, `valueColor`, `font` params. (for bg, you need the unicode character for # (%23) at the start of the hex code )

example usage: 
```markdown
![hits](https://nums-ten.vercel.app/badge?id=home&style=terminal&label=hits&bg=%23101414)```
```

#### Plain text number

```
https://nums-ten.vercel.app/count.txt?id=home
```

#### Example badge

Shields.io (recommended):
![hits](https://img.shields.io/endpoint?url=https%3A%2F%2Fnums-ten.vercel.app%2Fbadge.json%3Fid%3Dhome%26label%3Dhits%26cacheSeconds%3D30)

Direct terminal style:
![hits](https://nums-ten.vercel.app/badge?id=home&style=terminal&label=hits)

`/badge.json` returns the Shields.io endpoint schema:
```json
{"schemaVersion":1,"label":"hits","message":"123","color":"blue","cacheSeconds":30}
```

---
## Env Var Reference
| Var | Purpose |
|-----|---------|
| PORT | Local listen port (Go server) |
| SECRET_TOKEN | Auth shared secret (header `X-Auth-Token` or `?token=`) |
| PERSIST_FILE | File persistence (single legacy counter when Redis absent) |
| ALLOWED_ORIGINS | Comma list for CORS (e.g. https://site1,https://site2) |
| REDIS_URL | Full redis URL (overrides auto-build) |
| UPSTASH_REDIS_URL | Upstash host:port or redis(s):// URL |
| UPSTASH_REDIS_PASSWORD | Upstash password |
| REDIS_PREFIX | Key prefix (default hits:) |
| FAIL_FAST_REDIS | `1` to crash if Redis init fails |
| HIT_COUNTER_SECRET_TOKEN | Duplicate token for frontend build usage |
| NEXT_PUBLIC_HIT_COUNTER_URL | Public base URL of deployed counter service |

---
## Troubleshooting
- 401 Unauthorized: missing or wrong token.
- Not persisting: ensure Redis vars; check logs for `redis persistence enabled`.
- Need startup failure if Redis down: set `FAIL_FAST_REDIS=1`.
- Badge shows 0: call `/hit?id=...` first.
