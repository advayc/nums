# nums

<img width="1920" height="1093" alt="banner" src="https://github.com/user-attachments/assets/bd074a80-ea82-43a6-9649-bc00ab7d1446" />

nums is an open source hit counter and badge service for your website. It provides fast, serverless tracking using golang. the main purpose is to use it to increment and display website visits.

---

## usage

### 1. Fork & Clone

```bash
git clone https://github.com/advayc/nums.git
```

### 2. Create a Redis Database

upstash is recommended for pricing (not too good for scalability)

1. Go to [Upstash Redis](https://console.upstash.com/redis) and create a new database.
2. Copy the Endpoint (host:port) and password for use in your `.env` file.

### 3. Configure Environment Variables

Create a `.env` file with the following content

```env
PORT=8080
SECRET_TOKEN=YOUR_RANDOM_SECRET
PERSIST_FILE=/tmp/counter.txt
ALLOWED_ORIGINS=https://yourwebsite.com
REDIS_URL=
UPSTASH_REDIS_URL=
UPSTASH_REDIS_PASSWORD=
REDIS_PREFIX=hits:
FAIL_FAST_REDIS=0
HIT_COUNTER_SECRET_TOKEN=YOUR_RANDOM_SECRET
NEXT_PUBLIC_HIT_COUNTER_URL=https://your-deployment-url
```
(the private token can be anything)
**Minimum for persistence:** `SECRET_TOKEN` plus either `REDIS_URL` or both `UPSTASH_REDIS_URL` and `UPSTASH_REDIS_PASSWORD`.

### 4. Run Locally

```bash
go run ./cmd/server
curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:8080/hit?id=home"
curl -H "X-Auth-Token: $SECRET_TOKEN" "http://localhost:8080/count?id=home"
```

### 5. Deploy to Vercel

1. Import your fork into vercel
2. Add the environment variables in Vercel’s dashboard (settings -> environment variables)
3. Deploy; your base URL will be something like `https://<deployment>.vercel.app`.

---

## Endpoints

- `GET/POST /hit?id=foo`  
  Increments the counter for `foo` and returns `{ id, hits }`.  
  **Requires**: `X-Auth-Token` header or `?token=` param.

- `GET /count?id=foo`  
  Returns the current count as JSON: `{ id, hits }`.

- `GET /count.txt?id=foo`  
  Returns the count as plain text (good for direct badge usage).

- `GET /badge?id=foo&label=views`  
  Returns a live SVG badge (customizable via query params, does **NOT** increment).

- `GET /badge.json?id=foo&label=views`  
  Returns a Shields.io-compatible JSON schema for badges.

**Note:** Only `/hit` requires authentication. `/count`, `/count.txt`, `/badge`, and `/badge.json` are public.

---

## Badge Usage

### Markdown Shields.io Badge

```markdown
![hits](https://img.shields.io/endpoint?url=https%3A%2F%2F<your-vercel-deployment>.vercel.app%2Fbadge.json%3Fid%3Dhome%26label%3Dhits%26cacheSeconds%3D30)
```

- Change `label=hits` to customize the badge text.
- `cacheSeconds` controls update frequency (min 30s)

### Direct SVG Badge

```markdown
![hits](https://<your-vercel-deployment>.vercel.app/badge?id=home&style=terminal&label=hits)
```

- Customize label, style (`style=terminal` or default), background, and colors using `bg`, `labelColor`, `valueColor`, and `font` query params.
- Example with custom background:

```markdown
![hits](https://<your-vercel-deployment>.vercel.app/badge?id=home&style=terminal&label=hits&bg=%23101414)
```

### Plain Text Count

For integrating into scripts or as a simple counter:

```
https://<your-vercel-deployment>.vercel.app/count.txt?id=home
```

---

## client component implementation in nextjs

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

---

## Troubleshooting

- `401 Unauthorized`: Check your token.
- Not persisting: Ensure Redis vars are correctly set.
- Badge always 0: Call `/hit?id=...` at least once.
