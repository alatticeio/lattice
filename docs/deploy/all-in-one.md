# All-in-One Deployment

The all-in-one (`latticed`) deployment bundles NATS + SQLite + API + Dashboard into a single process — no external dependencies.

## Docker

```bash
docker run -d \
  --name latticed \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 4222:4222 \
  -v lattice-data:/app/data \
  -v $(pwd)/lattice.yaml:/etc/lattice/lattice.yaml \
  ghcr.io/alatticeio/latticed:latest
```

## Docker Compose

```yaml
version: "3"
services:
  latticed:
    image: ghcr.io/alatticeio/latticed:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "4222:4222"
    volumes:
      - lattice-data:/app/data
      - ./lattice.yaml:/etc/lattice/lattice.yaml

volumes:
  lattice-data:
```

## Configuration

Create `lattice.yaml`:

```yaml
app:
  listen: :8080
  name: "Lattice"
  env: "production"
  init_admins:
    - username: "admin"
      password: "changeme"        # ⚠ Change before deploying

jwt:
  secret: "replace-with-random-secret"   # ⚠ Use a 32-byte random value
  expire_hours: 24

signaling-url: "nats://localhost:4222"

database:
  dsn: "data/lattice.db"                # SQLite (default)
  # dsn: "root:pass@tcp(mariadb:3306)/lattice?charset=utf8mb4&parseTime=True"  # MariaDB
```
