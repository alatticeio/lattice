# Live Demo

Try Lattice from your browser — no installation required.

## Option 1: Docker (recommended)

```bash
curl -O https://raw.githubusercontent.com/alatticeio/lattice/main/docker-compose.demo.yml
docker compose -f docker-compose.demo.yml up -d
```

Then open [http://localhost:8080](http://localhost:8080).

**Default credentials:** `admin` / `lattice`

## Option 2: In-Browser Sandbox

::: tip Coming Soon
An interactive browser-based terminal is in development. You'll be able to try `lattice` CLI commands directly.
:::

## What to Try

1. **View the dashboard** — Your network at a glance
2. **Generate a node token** — Go to Manage → Tokens → Generate
3. **Join a node** — Run `lattice join --token wf_xxxx` on another machine
4. **Create a policy** — Go to Manage → Policies → Create
5. **Explore topology** — Go to Manage → Topology
