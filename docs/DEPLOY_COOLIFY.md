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

Do not select Dockerfile or a Docker build pack for this deployment. A binary
release is not built in Coolify, so passing secrets such as
`FIREBASE_SERVICE_ACCOUNT_JSON` through Docker build arguments can break JSON
parsing. Keep all runtime secrets in Coolify's environment settings instead;
the running binary receives them at runtime.

Use `/health` as the Coolify HTTP health-check path. Do not define a
Dockerfile `HEALTHCHECK` for the pre-built binary; Coolify should own the
HTTP readiness check and port routing.

Required environment variables include `DATABASE_URL`, `JWT_SECRET`,
`PORT`, and the optional SMS, Telegram, Firebase, and object-storage values.

For Firebase, prefer Base64 in Coolify:

```bash
base64 < firebase-service-account.json | tr -d '\n'
```

Store the output in `FIREBASE_SERVICE_ACCOUNT_BASE64` as a runtime environment
variable. The server also accepts the original JSON in
`FIREBASE_SERVICE_ACCOUNT_JSON` or a mounted JSON file via
`FIREBASE_CREDENTIALS_PATH`.
