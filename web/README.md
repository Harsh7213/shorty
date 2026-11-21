# Shorty web dashboard

The React/Vite frontend provides account sign-up and sign-in, workspace management, and a dashboard for creating, searching, and managing short links.

## Run locally

Start the API and database from the repository root with `docker compose up --build`, then run:

```bash
npm install
npm run dev
```

Copy `web/.env_example` to `web/.env` before running the frontend locally. Configure `VITE_API_PROXY_TARGET` there; it defaults to `http://localhost:8080`. The API path (`/api/v1`) and browser token storage key (`ACCESS_TOKEN`) are fixed in the frontend. In deployment, serve the frontend and reverse-proxy the API path to the backend on the same origin. The frontend never needs the backend JWT secret.

Build and run the production frontend with Docker Compose from the repository root:

```bash
docker compose up --build -d
```

The Compose frontend is served on port 80 at `http://localhost/` or `http://<server-host>/`; Nginx proxies `/api/v1` requests to the `backend` Compose service. Ensure TCP port 80 is allowed by the host firewall or cloud security group for remote access. While signed in, the dashboard refreshes account, workspace, link, analytics, and visible member data every 10 minutes without reloading the page. New accounts must verify their email before signing in. Organization owners and members with `organization.manage` can manage members and assign link or organization permissions from the User management view. Link analytics are fetched from the backend API and shown per link. Active links can also download a QR code from the authenticated QR endpoint.
