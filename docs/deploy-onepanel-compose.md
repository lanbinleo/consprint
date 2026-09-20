# Docker Compose Deployment

This guide deploys AP Psych Final Sprint as one Docker service. The app uses SQLite, so the only persistent runtime directory is `data/`.

## Current Production Deployment (since 2026-09-20)

- Server: `root@43.156.135.80` (CentOS 7 VM, 2 vCPU / 3 GB RAM, OnePanel stack; SSH key auth).
- App directory: `/opt/1panel/apps/mypsych` — a git clone of this repository; the container is the OnePanel compose project `mypsych` (container name `ap-psych-final`).
- Public entry: `https://psy.tsinglan.top` (OnePanel openresty reverse proxy) → host port `8052` → container port `8052` (`.env` sets `PORT=8052`).
- The server's `compose.yaml` differs from the repo copy only in the port mapping and healthcheck port (8052 vs 8080). `git reset --hard` reverts it — restore from `/root/mypsych-backups/compose.yaml.8052` afterwards.
- The server `.env` (gitignored) holds production secrets: `JWT_SECRET`, `REGISTRATION_INVITE_CODE`, `CORS_ORIGIN=https://psy.tsinglan.top`, and `ADMIN_EMAILS=leo.huo_27@tsinglan.org`. Production mode has no first-user-admin fallback, so `ADMIN_EMAILS` is required or nobody can manage the site.
- `data/public/ap-psych-sample.pdf` backs the demo Notes PDF tab. It is gitignored — restore it manually on a fresh checkout.
- Entra SSO server-side config added 2026-09-20: `ENTRA_*` block in the server `.env` with production redirect `https://psy.tsinglan.top/api/auth/entra/callback`, `ENTRA_ALLOWED_DOMAINS=tsinglan.org`, `ENTRA_FRONTEND_REDIRECT=/auth/callback`; `/api/meta` returns `entra:true`. Cloud side: the same production redirect URI must exist in the Entra app registration (Authentication → Web redirect URIs) or Microsoft rejects the sign-in with AADSTS50011.
- Update flow from a dev machine: commit + `git push origin master`, then on the server: keep `.env` and the 8052 compose variant, `git fetch && git reset --hard origin/master`, restore the compose variant (`\cp -f /root/mypsych-backups/compose.yaml.8052 compose.yaml` — plain `cp` is aliased to `cp -i` and blocks on the overwrite prompt), optionally wipe `data/app.db*` for a fresh database, then `docker compose up -d --build`.
- Pre-wipe database/compose backups live under `/root/mypsych-backups/`.

Fresh-database seeds (verified 2026-09-20): 794 concepts, all sourced from `cards.compact`; 0 announcements (the welcome-announcement seed was removed on purpose); 0 questions / practice sets (the sample question bank is never auto-imported — teachers import via the admin two-step).

## What Persists

The Compose file mounts:

```text
./data:/app/data
```

That means:

- `data/app.db` is the live database.
- `data/sources/` contains the raw source files and enrichment data.
- You can upload your local `data/` directory to the server and the container will use it directly.
- Rebuilding or replacing the image will not delete user accounts, ratings, review events, or imported content.

## Server Steps

1. SSH into the server and choose an app directory.

```bash
mkdir -p /opt/ap-psych-final
cd /opt/ap-psych-final
```

2. Clone the GitHub repository.

```bash
git clone YOUR_GITHUB_REPO_URL .
```

3. Upload your local `data/` directory to the server app directory.

After upload, the server should look like this:

```text
/opt/ap-psych-final/
  compose.yaml
  Dockerfile
  data/
    app.db
    sources/
```

If you do not upload `data/app.db`, the app will create a new database on first startup and import from `data/sources/`.

4. Create the server `.env`.

```bash
cp .env.example .env
nano .env
```

Required production values:

```env
APP_ENV=production
GIN_MODE=release
HOST=0.0.0.0
PORT=8080
JWT_SECRET=replace-with-a-long-random-secret-at-least-32-chars
REGISTRATION_INVITE_CODE=change-me-class-code
APP_TIMEZONE=Asia/Shanghai
APP_USE_SYSTEM_TIMEZONE=false
CORS_ORIGIN=https://your-domain.example
```

Use a unique random `JWT_SECRET`. In production mode, the server refuses to start if this is missing or too short.

5. Build and start.

```bash
docker compose up -d --build
```

6. Check logs and health.

```bash
docker compose logs -f
curl http://127.0.0.1:8080/api/health
```

Expected health response:

```json
{"ok":true}
```

## OnePanel Flow

1. Open OnePanel.
2. Create or choose a website/domain.
3. Create a Compose app/project pointing at the repository directory, or paste the contents of `compose.yaml`.
4. Make sure the project working directory contains the uploaded `data/` folder and the `.env` file.
5. Start the Compose project.
6. In OnePanel reverse proxy settings, proxy the domain to the app service on port `8080`.
7. Enable HTTPS for the domain.

## Updating

From the server directory:

```bash
git pull
docker compose up -d --build
```

The `data/` bind mount is preserved.

## Backup

Stop the service before copying SQLite for the cleanest backup:

```bash
docker compose stop
tar -czf ap-psych-data-backup.tgz data
docker compose start
```

For a quick emergency backup while running:

```bash
cp data/app.db data/app.db.backup
```

## Useful Commands

```bash
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
docker compose up -d --build
```

## Notes

- Do not commit `.env` or `data/app.db`.
- The first registered account in a new database becomes admin.
- If you upload an existing `data/app.db`, existing users and roles are preserved.
- If `REGISTRATION_INVITE_CODE` is set, new classmates must enter that code to register.
