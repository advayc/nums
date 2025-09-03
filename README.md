# nums

[<img width="1920" height="1093" alt="banner" src="https://github.com/user-attachments/assets/bd074a80-ea82-43a6-9649-bc00ab7d1446" />](https://docs.advay.ca)

nums is an open source hit counter and badge service for your website. It provides fast, serverless tracking using golang. the main purpose is to use it to increment and display website visits.

---

View the full documentation and api reference [here](https://docs.advay.ca)
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

---

## Documentation

- Hosted docs: https://docs.advay.ca
- Guide + API reference (Mintlify) in `docs/` (open locally with `mint dev`).
