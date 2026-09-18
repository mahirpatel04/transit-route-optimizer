# Transit Route Optimizer — Frontend

A small React (Vite) client for the [transit-route-optimizer](../) API. Enter a "from" and "to" NYC address (or click a suggested landmark) and it finds the fastest real subway route between them — station names, which line to board, and the walks at each end included. Also shows the last GTFS ingest time in the header.

## Configuration

`VITE_API_URL` (in `.env`, committed — it's build-time config, not a secret) points at the backend's Lambda Function URL.

## Development

```bash
npm install
npm run dev
```

## Testing

```bash
npm test
```

## Build

```bash
npm run build
```

Deployed automatically to GitHub Pages on push to `main` via `.github/workflows/deploy-frontend.yml`.
