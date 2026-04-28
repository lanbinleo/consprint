# Docker Compose Deployment

This guide deploys AP Psych Final Sprint as one Docker service. The app uses SQLite, so the only persistent runtime directory is `data/`.

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
