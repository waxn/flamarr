# Flamarr — Run & Deploy

This README explains how to run Flamarr locally, in Docker, and how to push images to Docker Hub via GitHub Actions.

## Prerequisites
- Go 1.22+ (for local builds)
- Docker & docker-compose (optional)
- Git

## Run locally (development)

1. Build the binary:

```bash
go build -o flamarr ./
```

2. Create a writable data directory (the app stores SQLite DB there):

```bash
mkdir -p ./data
```

3. Run the app (environment vars optional):

```bash
PORT=5005 DATA_DIR=$(pwd)/data ./flamarr
# or during development
go run main.go
```

Open http://localhost:5005 in your browser.

## Using Docker

Build the image locally:

```bash
docker build -t flamarr:local .
```

Run with a host-mounted data directory:

```bash
docker run --rm -p 5005:5005 \
  -v $(pwd)/data:/data \
  -e PORT=5005 -e DATA_DIR=/data \
  flamarr:local
```

Using docker-compose (if present):

```bash
docker compose up --build -d
```

Notes:
- Ensure the `data` directory is writable by the container process. If you see permission issues, adjust ownership or use a Docker volume.

## Push to Docker Hub (manual)

Tag and push manually:

```bash
docker tag flamarr:local <your-docker-username>/flamarr:latest
docker push <your-docker-username>/flamarr:latest
```

## GitHub Actions — Build & Push to Docker Hub

The repository includes a workflow at `.github/workflows/docker-image.yml`. To enable automatic push to Docker Hub:

1. In your GitHub repository, go to Settings → Secrets → Actions and add two secrets:
   - `DOCKERHUB_USERNAME` — your Docker Hub username
   - `DOCKERHUB_TOKEN` — a Docker Hub access token (create this in Docker Hub Account → Security → New Access Token)

2. Ensure the workflow file contains the `docker/login-action` and `docker/build-push-action` steps. When secrets are present, pushing to `master`/`main` will build and push images to `docker.io/<username>/flamarr`.

3. After pushing, check the Actions tab in GitHub for job logs. The `Log in to Docker Hub` and `Build and push image` steps should succeed.

If the workflow did not run or did not push:
- Verify secrets are configured correctly.
- Open the workflow run logs and inspect the `docker/login-action` output for authentication errors.
- Confirm the workflow file is on the branch you pushed.

## Resolving the recent workflow conflict

If you ran into a merge/rebase conflict where the Dockerfile contents ended up in `.github/workflows/docker-image.yml`, resolve it by editing the file and removing conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) so the file contains the YAML workflow, then:

```bash
git add .github/workflows/docker-image.yml
git rebase --continue   # or `git commit` if you are merging
git push
```

If you prefer to abort the rebase and start over:

```bash
git rebase --abort
```

## Troubleshooting
- If the app exits immediately, check stdout for stack traces and ensure `DATA_DIR` points to a writable directory.
- For Docker permission issues, try using a named volume instead of mounting a host folder.
- If GitHub Actions fails to push, check that `DOCKERHUB_TOKEN` is valid and has push permissions.

## Useful commands

```bash
# build locally
go build ./...

# run locally
go run main.go

# build docker image
docker build -t flamarr:local .

# tag & push manually
docker tag flamarr:local <your-docker-username>/flamarr:latest
docker push <your-docker-username>/flamarr:latest
```

---
If you'd like, I can commit & push this README for you, or I can also update the workflow file here to use `docker/build-push-action` and verify its syntax locally. Which do you prefer?
