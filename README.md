# barivara-server

Backend API for Barivara. Built with Go + Gin, backed by Postgres (Neon)
and Neon Object Storage. Runs as a plain long-lived binary — deploy it on
your own VPS behind a reverse proxy (no serverless platform).

## Features

- Phone + password auth (JWT; OTP/refresh is deferred for later)
- Full CRUD for properties, units, renters, invoices
- Tenant self-serve linking: a renter record gets a join code, the tenant
  enters it, the landlord approves (or the landlord links a tenant account
  directly by email)
- Presigned upload/download URLs against Neon Object Storage
- SMS rent reminders / payment receipts (`bulksmsbd.net`)
- `GET /api/cron/check-reminders` — call this from your own cron (e.g. a
  system crontab or systemd timer hitting it with `curl`) since there's no
  platform cron here anymore

## Setup

### Prerequisites

- Go 1.21+
- A Neon project with Postgres + Object Storage provisioned (`neon link`,
  see the `neon` CLI) — `DATABASE_URL` and the `AWS_*` object storage
  credentials are pulled by `neon env pull` into `.env`
- bulksmsbd.net account with API key

### Environment

`.env` (gitignored; `neon link`/`neon env pull` write most of this for you):

```env
BULKSMSBD_API_KEY=your_api_key
BULKSMSBD_SENDER_ID=your_sender_id
JWT_SECRET=a_long_random_secret
PORT=8080

# optional — Telegram notification to admin on new landlord requests
TELEGRAM_BOT_TOKEN=
TELEGRAM_ADMIN_CHAT_ID=

# optional — FCM push to the Android app (landlord-approval notification)
FIREBASE_SERVICE_ACCOUNT_JSON=
# or use FIREBASE_CREDENTIALS_PATH=/absolute/path/to/firebase-key.json

# from `neon env pull`
DATABASE_URL=postgres://...
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_ENDPOINT_URL_S3=...
AWS_REGION=...
OBJECT_STORAGE_BUCKET=assets
```

### Run locally

```bash
go run cmd/server/main.go
```

### Deploy to a VPS

```bash
go build -o server ./cmd/server
# copy `server` + `.env` to the VPS, run it behind nginx/caddy,
# and manage it with systemd (or your process manager of choice).
```

## API

Full route list: `internal/server/routes.go`. Highlights:

- `POST /api/auth/signup`, `POST /api/auth/signin`, `GET /api/me`
- `GET/POST /api/properties`, `.../units`, `.../renters`, `.../invoices`
- `POST /api/properties/:id/units/:id/renters/:id/join-code` — tenant
  linking (landlord side)
- `POST /api/tenant/link-requests`, `GET /api/tenant/renter`,
  `GET /api/tenant/invoices` — tenant linking (tenant side)
- `POST /api/uploads/presign`, `GET /api/uploads/resolve`
- `POST /api/send-reminder`, `POST /api/send-receipt`,
  `GET /api/cron/check-reminders`
- `GET /api/invoices/:invoiceId` — public, unauthenticated invoice link
  (what the SMS templates below point to)

## Templates

**Reminder:**
```
Dear {name},
Your rent of {amount} BDT for {monthYear} at {propertyName} is due.
View invoice: https://barivarabd.web.app/invoice/{invoiceId}
Please pay on time. Thank you.
```

**Receipt:**
```
Dear {name},
Thank you! Payment of {amount} BDT for {monthYear} at {propertyName} received.
Receipt: https://barivarabd.web.app/invoice/{invoiceId}
Have a great month!
```

## Project structure

```
barivara-server/
├── cmd/server/main.go       # entrypoint (go run / go build)
├── internal/
│   ├── auth/                # bcrypt + JWT
│   ├── db/                  # Postgres pool + embedded schema.sql
│   ├── middleware/          # RequireAuth / RequireRole
│   ├── models/              # domain structs shared by store + handlers
│   ├── store/                # all Postgres queries
│   ├── storage/             # Neon Object Storage (presigned S3 URLs)
│   ├── handler/              # HTTP handlers (gin)
│   ├── sms/                  # bulksmsbd.net HTTP caller
│   ├── server/               # router + wiring
│   └── types/                # request/response DTOs
├── go.mod / go.sum
├── .env.example
└── README.md
```
