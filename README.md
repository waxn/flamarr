# Flamarr

A self-hosted dashboard for your services and bookmarks. Built with Go + SQLite, runs in Docker.

**Features**
- Single-user account created on first visit
- Add services (self-hosted apps) and bookmarks with `A`, `S`, or `B`
- Live clock and time-based greeting (morning / afternoon / evening / night)
- Weather widget via [Open-Meteo](https://open-meteo.com/) — no API key, location saved to DB and shared across all devices
- Real-time search, inline edit/delete on hover
- All data stored in SQLite at `DATA_DIR/flamarr.db`

---

## Run with Docker Compose (recommended)

```bash
docker compose up -d
```

Open [http://localhost:5005](http://localhost:5005). Create your account on first visit.

Data is persisted in a named Docker volume (`flamarr_data`).

---

## Run locally

```bash
go run .
# or build first
go build -o flamarr .
./flamarr
```

Open [http://localhost:5005](http://localhost:5005).

Environment variables (all optional):

| Variable   | Default | Description                        |
|------------|---------|------------------------------------|
| `PORT`     | `5005`  | Port to listen on                  |
| `DATA_DIR` | `data`  | Directory for the SQLite database  |

---

## Pull from Docker Hub

The image is published at `waxn/flamarr`. To run without building locally:

```bash
docker run -d \
  -p 5005:5005 \
  -v flamarr_data:/data \
  --name flamarr \
  --restart unless-stopped \
  waxn/flamarr:latest
```

Or with Compose, update `docker-compose.yml` to pull instead of build:

```yaml
services:
  flamarr:
    image: waxn/flamarr:latest
    container_name: flamarr
    restart: unless-stopped
    ports:
      - "5005:5005"
    volumes:
      - flamarr_data:/data

volumes:
  flamarr_data:
```

Then:

```bash
docker compose up -d
```

## Build and push your own image

```bash
docker build -t yourusername/flamarr:latest .
docker push yourusername/flamarr:latest
```

Or use the included GitHub Actions workflow (`.github/workflows/docker-image.yml`). Add two secrets to your repo:
- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN` (create in Docker Hub → Account Security → New Access Token)

Pushing to `main` or `master` will build and push automatically.

---

## Keyboard shortcuts

| Key       | Action                        |
|-----------|-------------------------------|
| `A` / `N` | Open add modal (Service)      |
| `S`       | Open add modal (Service)      |
| `B`       | Open add modal (Bookmark)     |
| `Enter`   | Save (while modal is open)    |
| `Esc`     | Close modal                   |
