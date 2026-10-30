# Coolify Deploy

## How it works

Build the Linux/AMD64 binary locally, commit it, and push it to `main`.
Coolify pulls and runs the pre-built binary without compiling the source.

## Deploy

```bash
make build
git add barivara-server
git commit -m "build: deploy"
git push origin main
```

The binary is intentionally tracked in Git. Do not add it to `.gitignore`.

## Coolify setup

| Setting | Value |
|---|---|
| Source | Git |
| Branch | `main` |
| Build method | None (pre-compiled binary) |
| Start command | `./barivara-server` |
| Port | `8080` |
| Environment variables | Configure in the Coolify dashboard |

Required environment variables include `DATABASE_URL`, `JWT_SECRET`,
`PORT`, and the optional SMS, Telegram, Firebase, and object-storage values.
