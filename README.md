# linkany

## introduction

linkany can create your own secure private network based on wireguard, and also provide a web ui to manage the network.
linkany can connect multiple devices, and you can manage the devices through the web ui. linkany can also provide access
control for you own secure private network.

## Technology

## Network Topology

## Quick start

- First, you need register an account on linkany, and then login.
- Second, you need create a network, and then you can configure the network on the ui.
- Third, download the wireguard, run the wireguard on you devices,login with the account you registered, and then you
  can connect to the network.

## Installation

serval way to install linkany, you can choose the way you like.

### Docker

```bash
docker run -d --privilege --name linkany -p 51820:51820/udp linkany/linkany
```

### Binary

```bash
bash <(curl -s https://linkany.io/install.sh)
```

### Source

```bash
git clone https://github.com/linkanyio/linkany.git
make build && install
```

### App

Download the app from the [linkany.io](https://linkany.io) and install it.

### Nas

## About relay

if direct connect to the network failed, you can use relay to connect to the network. We provide a free relay server for
relaying your data, but you can also use your own relay server. We also provide a relay image to help you deploy the
relay server easily, also you can use coturn[] which is a great turn server.

## How to deploy relay

## Features

- [x] Configure-free: Just register an account and login, and then you can create you own network.
- [x] Secure: Based on wireguard, and provide access control.
- [x] Access control: You can control the access of the network, building rules you like.
- [x] Web ui: Provide a web ui to manage the network.
- [x] Relay: Provide relay service to help you connect to the network, once you can't connect to the network directly(
  P2P).
- [x] Multi-platform: Support multiple platforms, such as windows, linux, macos, android, ios, nas...
- [] Metrics: Provide metrics for the network, such as traffic, connection, etc.
- [] Multi-network: You can create multiple networks, and manage them through the web ui.
- [] Docker: Provide docker image to help you deploy the network easily, also have a web ui for docker client, need not
  install pc app.
- [] DNS: Provide dns service for the network, you can use you own dns domain to access the network.

## License

Apache License 2.0



