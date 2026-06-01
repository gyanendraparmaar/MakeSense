# FreeLLMAPI (config only)

MakeSense runs FreeLLMAPI from the **published Docker image** — this folder holds only local config (`freellmapi/.env`), not the upstream source.

```bash
# From repo root
cp freellmapi/.env.example freellmapi/.env
# Set ENCRYPTION_KEY in freellmapi/.env (see .env.example)
docker compose up -d freellmapi
open http://localhost:3001
```

- **Upstream repo:** https://github.com/tashfeenahmed/freellmapi  
- **MakeSense integration guide:** [`FREELLMAPI.md`](../FREELLMAPI.md)
