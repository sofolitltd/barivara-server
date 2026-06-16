# barivara-server

Rent reminder SMS server for Barivara. Built with Go + Gin. Runs on Vercel.

## Features

- **Auto reminder SMS** — Vercel cron sends rent reminders on the 3rd and 5th of each month
- **Manual reminder** — Frontend triggers `POST /api/send-reminder` per renter
- **Payment receipt** — Frontend triggers `POST /api/send-receipt` after payment

## Setup

### Prerequisites

- Go 1.21+
- Firebase project with Firestore (`renters`, `invoices`, `properties`, `sms_logs` collections)
- bulksmsbd.net account with API key

### Environment

Copy `.env.example` to `.env` and fill in:

```env
BULKSMSBD_API_KEY=your_api_key
BULKSMSBD_SENDER_ID=your_sender_id
FIREBASE_CREDENTIALS={"type":"service_account","project_id":"..."}
PORT=8080
```

### Run locally

```bash
go run cmd/server/main.go
```

## API

### POST /api/send-reminder

Send rent reminder to a renter.

```json
{"renterId": "abc123", "invoiceId": "inv456"}
```

### POST /api/send-receipt

Send payment confirmation to a renter.

```json
{"renterId": "abc123", "invoiceId": "inv456"}
```

### GET /api/cron/check-reminders

Called daily by Vercel cron. Queries all properties where `reminderDay == today`, then for each property finds active renters with unpaid invoices and sends SMS reminders.

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

## Deploy to Vercel

```bash
# Install Vercel CLI
npm i -g vercel

# Set environment variables
vercel env add BULKSMSBD_API_KEY
vercel env add BULKSMSBD_SENDER_ID
vercel env add FIREBASE_CREDENTIALS

# Deploy
vercel --prod
```

## Firestore collections

| Collection | Purpose |
|-----------|---------|
| `renters` | Renter info: name, phone, propertyId, isActive |
| `invoices` | Invoice info: renterId, totalAmount, monthYear, status |
| `properties` | Property info: name, address |
| `sms_logs` | Auto-created: log of every SMS sent |

## Project structure

```
barivara-server/
├── api/
│   └── index.go            # Vercel serverless entry point
├── cmd/
│   └── server/
│       └── main.go         # Local dev server entry point
├── internal/
│   ├── firebase/
│   │   └── firestore.go    # Firestore queries + SMS logging
│   ├── handler/
│   │   ├── handler.go
│   │   ├── cron.go
│   │   ├── reminder.go
│   │   └── receipt.go
│   ├── sms/
│   │   └── sender.go       # bulksmsbd.net HTTP caller
│   └── types/
│       └── types.go
├── vercel.json             # Cron schedule + rewrite config
├── go.mod / go.sum
├── .env.example
└── README.md
```

