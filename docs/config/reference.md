# Configuration Reference

## Agent Config (`~/.lattice/lattice.yaml`)

The agent configuration file is created by `lattice init` and read by `lattice up`.

```yaml
server-url: "http://lattice.example.com:8080"
token: "eyJhbGci..."
relay-url: "relay.example.com:6266"
relay-quic-url: "relay.example.com:6267"
```

| Key | Required | Description |
|-----|----------|-------------|
| `server-url` | Yes | Management server HTTP URL |
| `token` | Yes | Enrollment token |
| `relay-url` | No | TCP relay server address |
| `relay-quic-url` | No | QUIC relay server address |

> **Note:** `signaling-url` (NATS URL) is auto-discovered from `server-url` via `/api/v1/discovery`. You can still set it manually to override the discovered value.

## Server Config (`/etc/lattice/lattice.yaml`)

The all-in-one server (`latticed`) configuration file.

### App

```yaml
app:
  listen: :8080           # API listen address
  name: "Lattice"         # Display name
  env: "production"       # Environment (development/production)
  init_admins:            # Initial admin accounts
    - username: "admin"
      password: "changeme"
```

### JWT

```yaml
jwt:
  secret: "random-32-byte-secret"  # JWT signing secret
  expire_hours: 24                  # Token lifetime
```

### Signaling

```yaml
signaling-url: "nats://localhost:4222"  # NATS server URL
```

### Database

```yaml
database:
  dsn: "data/lattice.db"              # SQLite (default)
  # dsn: "user:pass@tcp(host:3306)/lattice?charset=utf8mb4&parseTime=True"  # MySQL/MariaDB
```

### Monitoring (Pro)

```yaml
monitor:
  address: "http://victoria-metrics:8428"
```

### License (Pro)

```yaml
license:
  file: "/etc/lattice/license.jwt"
```
