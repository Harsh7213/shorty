 <h1>
  <img src="web/public/banner.svg" alt="Shorty" height="80" />
</h1>

Less URL. More impact.

Shorty is a simple, fast, and modern URL shortener that transforms long, messy URLs into clean shareable links.

Because nobody likes sharing ridiculously long URLs.

## ✨ Features
 URL Shortening — Turn lengthy URLs into short, shareable links.

 Fast Redirection — Redirect users to their destination without unnecessary friction.

 Clean & Simple — Minimal interface designed to make link shortening effortless.

 Link Analytics — Track link performance and engagement .

## 🚀 Why Shorty?

Stand out from the bulk.

Sharing links should be simple.

Shorty makes it easier to share URLs across social media, messages, emails, and anywhere else you need a clean shareable link.

No more endless URLs. Just short, simple links.


### ⚙️ Getting Started

Clone the repository:

```bash
git clone https://github.com/Harsh7213/shorty.git
```

Navigate into the project:

```bash
cd shorty
docker compose up --build -d
```

Compose starts PostgreSQL, shorty-backend , shorty-frontend, RabbitMQ and Mailpit. The dashboard is available on port 80 at `http://localhost/` or `http://<server-host>/`; The local database is initialized automatically and stored in a named volume. The backend container receives its database connection and local Mailpit settings from Compose; `backend/.env` supplies the JWT secret and sender address. Registration and invitation requests commit an encrypted email job to the database outbox and return without waiting for SMTP. The dispatcher publishes jobs to RabbitMQ, and the worker retries SMTP delivery with exponential delays of 30 seconds, 1, 2, and 4 minutes; after five failed attempts, jobs move to the durable `shorty.email.dead` queue. To retry a failed job after fixing SMTP, get its ID with `SELECT id, last_error FROM shorty.email_outbox WHERE failed_at IS NOT NULL AND delivered_at IS NULL`, then run `go run . retry-failed-email <id>` from `backend/`. Mailpit accepts verification messages without delivering real email at `http://localhost:8025`.

For production, create and populate separate ignored files from the production examples:

```bash
cp -n env.production_example .env.production
cp -n backend/env.production_example backend/.env.production
```

Replace every `REPLACE_...` value. Generate secrets with `openssl rand -base64 48` for `JWT_SECRET`, `openssl rand -base64 32` for `EMAIL_ENCRYPTION_KEY`, and `openssl rand -hex 24` for the database and RabbitMQ passwords. Set your real public HTTPS URL and SMTP provider details. Then start the production-configured stack:

```bash
docker compose --env-file .env.production -f compose.yaml -f compose.production.yaml up --build -d
```

The production override requires public URL, SMTP, and RabbitMQ settings and disables Mailpit. Keep `EMAIL_ENCRYPTION_KEY` stable and the same for the API and notification worker; the development fallback derives it from `JWT_SECRET`. The Compose RabbitMQ service is a single persistent broker; use a managed or clustered broker for high availability.

To run the backend directly on your machine instead, start Mailpit, RabbitMQ, and PostgreSQL, configure `DATABASE_URL` and `RABBITMQ_URL` in `backend/.env`, then run `go run .` from `backend/`. Run `go run . outbox-dispatcher` and `go run . notification-worker` in separate processes to deliver queued email.

### ⚙️ Getting Started

Clone the repository and start everything:

```bash
git clone https://github.com/Harsh7213/shorty.git
cd shorty
docker compose up --build -d
```

Compose starts PostgreSQL, shorty-backend, shorty-frontend, RabbitMQ, and Mailpit.
The dashboard is at `http://localhost/`; Mailpit is at `http://localhost:8025`.

**How email delivery works.** The backend is a single process that runs the API,
the outbox dispatcher, and the email worker. Registration and invitation requests
write an encrypted job to the database outbox and return immediately. The
dispatcher publishes jobs to RabbitMQ, and the worker sends them over SMTP. Failed
sends are retried after 30 seconds, then 1, 2, and 4 minutes. After five failed
attempts, the job moves to the durable `shorty.email.dead` queue. If RabbitMQ is
down, queued emails are sent automatically once it returns. 

**Running without Docker.** Start PostgreSQL, RabbitMQ, and Mailpit, set
`DATABASE_URL`, `RABBITMQ_URL`, and `JWT_SECRET` (or `EMAIL_ENCRYPTION_KEY`) in
`backend/.env`, then run `go run .` from `backend/`. This one command starts the
API, dispatcher, and worker.

### 🔐 API 

With the backend running, open the interactive API documentation at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html). 


### 🤝 Contributing

Contributions, ideas, and feedback are always welcome!

Feel free to fork the repository, open an issue, or submit a pull request.

### 📄 License

This project is open source. See the LICENSE file for details.

<p align="center">
**Shorty — Less URL. More impact.**
</p>