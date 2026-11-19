# 🔗 Shorty
Less URL. More impact.

Shorty is a simple, fast, and modern URL shortener that transforms long, messy URLs into clean shareable links.

Because nobody likes sharing ridiculously long URLs. 😉

## ✨ Features
🔗 URL Shortening — Turn lengthy URLs into short, shareable links.
⚡ Fast Redirection — Redirect users to their destination without unnecessary friction.
🎯 Clean & Simple — Minimal interface designed to make link shortening effortless.
📊 Link Analytics — Track link performance and engagement .
📱 Responsive Design — Works smoothly across devices.

## 🚀 Why Shorty?

Stand out from the bulk.

Sharing links should be simple.

Shorty makes it easier to share URLs across social media, messages, emails, and anywhere else you need a clean link.

No more endless URLs. Just short, simple links.

### 🛠️ Tech Stack

Built with modern web technologies.

go

### ⚙️ Getting Started

Clone the repository:

```bash
git clone https://github.com/Harsh7213/shorty.git
```

Navigate into the project:

```bash
cd shorty
```

Configure [backend/.env](./backend/.env) with the JWT and SMTP sender settings below, then start the backend and PostgreSQL in Docker:

```bash
docker compose up --build -d
```

Compose starts PostgreSQL, the backend API, the frontend, and Mailpit. The dashboard is available on port 80 at `http://localhost/` or `http://<server-host>/`; it proxies API requests to the backend through the Compose network. The local database is initialized automatically and stored in a named volume. The backend container receives its database connection and local Mailpit settings from Compose; set `JWT_SECRET` (at least 32 random bytes) and `SMTP_FROM` in `backend/.env`. Mailpit accepts verification messages without delivering real email at `http://localhost:8025`.

To run the backend directly on your machine instead, start Mailpit with `docker compose up -d mailpit`, configure `DATABASE_URL` in `backend/.env` for a local PostgreSQL instance, and run `go run .` from `backend/`.

### 📚 Swagger API docs

With the backend running, open the interactive API documentation at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html). Use `POST /api/v1/auth/login` to get an access token; click **Authorize** and enter `Bearer <access_token>` to try protected endpoints. After changing API annotations, install the matching generator and regenerate the committed specification:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd backend
swag init -g main.go
```

### 🔐 Backend API flow

The backend uses email/password accounts, email verification, and short-lived JWT bearer tokens. Before starting it, configure the backend `.env`:

```env
PORT=8080
DOMAIN=http://localhost:8080/
DATABASE_URL=postgres://user:password@localhost:5432/shorty_db?sslmode=disable
JWT_SECRET=<at-least-32-random-bytes>
PUBLIC_API_URL=http://localhost:8080
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_STARTTLS=false
SMTP_FROM=shorty@localhost
```

`SMTP_STARTTLS` defaults to `true`; disabling it is allowed only for `localhost`, a loopback IP, or the local Docker Mailpit service. For real email delivery, use your provider's SMTP host and port with STARTTLS enabled, and set `SMTP_USERNAME` and `SMTP_PASSWORD` if required. Do not commit `.env` or expose `JWT_SECRET` to the frontend.

The embedded SQL creates tables when absent and migrates the owner and link-attribution columns. Existing organizations receive the earliest-joined member with `organization.manage` as owner. The migration requires each existing organization to have such a manager. Use a fresh development database for unrelated older links/membership schema changes, or prepare and apply a data migration before deploying over an existing database.
Each account can belong to only one organization. If an existing database has users in multiple organizations, consolidate those memberships before starting the updated backend so the unique membership index can be created.

1. **Register:** `POST /api/v1/auth/register` with `{"email":"person@example.com","password":"at-least-12-characters","display_name":"Person"}`. The backend stores a bcrypt password hash and sends a one-time email verification link.
2. **Verify:** open the link sent by email. It calls `GET /api/v1/auth/verify?token=...`. If delivery fails, request another email with `POST /api/v1/auth/resend-verification` and `{"email":"person@example.com"}`.
3. **Sign in:** `POST /api/v1/auth/login` with email and password. The response includes a 15-minute JWT; sign in again when it expires. Send it on protected requests as `Authorization: Bearer <access_token>`.
4. **Create an organization:** `POST /api/v1/org` with `{"name":"Example"}`. The signed-in user becomes its owner, a separate superuser role with full organization access; owners are not admins and their privileges do not appear in the member permissions array.
5. **Create links:** `POST /api/v1/link` with `{"organization_id":1,"long_url":"https://example.com","expires_at":"2027-01-01T00:00:00Z"}`. `expires_at` is optional; omit or pass `null` for no expiry. Submitting the same URL with the same expiry as an active, unexpired link in that organization returns its existing short URL; a different expiry creates a separate link.
6. **List links:** `GET /api/v1/org/{organizationID}/links` returns the organization’s links, click counts, accessed country codes, expiry dates, and each creator’s user ID, email, and display name to members with `link.manage`. Country codes are recorded when the redirecting request includes the Cloudflare `CF-IPCountry` header; other requests are retained without a country.
7. **View analytics:** `GET /api/v1/org/{organizationID}/analytics?period=week` returns daily visit counts for each organization link for the last seven days. Use `period=month` for the last 30 days. Both require `link.manage`.
8. **Manage links:** `PATCH /api/v1/link/{code}` with a `status` of `active` or `disabled`, and/or an `expires_at` timestamp or `null`. Expired links cannot be disabled. `DELETE /api/v1/link/{code}` revokes the link. Revoked links cannot be reactivated.
9. **Share an organization:** an organization manager can add an existing account or invite a new user with `POST /api/v1/org/{organizationID}/members` and `{"email":"person@example.com","password":"password","permissions":["link.manage"]}`. The `password` field is required only when the email does not already belong to an account; omit it when adding an existing account. For a new account, it must be 12–72 bytes, is hashed, and is never sent by email. After opening the invitation link to verify their email, the new user signs in with the temporary password provided by the organization administrator. They can then change it with `PATCH /api/v1/auth/password` using `{"password":"password","new_password":"a-new-password"}` and their bearer token. If email delivery fails, the manager can retry with `POST /api/v1/auth/resend-verification` and `{"email":"person@example.com"}`. Owners have full access independently of member permissions. Managers can list members with `GET /api/v1/org/{organizationID}/members`; the response marks the owner with `is_owner`. They can remove a user's membership with `DELETE /api/v1/org/{organizationID}/members/{userID}`. Removing membership leaves their account, links, and click history intact. The owner cannot be removed as a member, and only the owner can remove the last active organization manager.
10. **Account lifecycle:** `GET /api/v1/auth/me` returns the signed-in account. `DELETE /api/v1/auth/account` permanently deletes the signed-in account. If they own an organization, that organization, its members' accounts, memberships, links, and click history are deleted; this also applies recursively to organizations owned by those members. Otherwise, only their membership is removed while links and click history are preserved, with creator email and display name retained for attribution.

Public short links redirect only while active and unexpired. `GET /{code}` performs the redirect, and `GET /api/v1/qr/{code}` generates a QR code for an active, unexpired link.

All organization members can view workspace links and analytics in the dashboard. The `link.manage` permission is required to create, update, disable, or revoke links; organization owners retain full access.

### 🗺️ Roadmap
Custom short URLs
QR code generation
Link analytics
Link expiration
Dashboard for managing links
API access

### 🤝 Contributing

Contributions, ideas, and feedback are always welcome!

Feel free to fork the repository, open an issue, or submit a pull request.

### 📄 License

This project is open source. See the LICENSE file for details.

<p align="center">
**Shorty — Less URL. More impact.**
</p>