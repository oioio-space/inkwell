# Hosting a relay

inkwell has no server: people who edit together connect to each other. Most of
the time that works directly, even from behind a home router, thanks to hole
punching. When it cannot (two strict routers, a company network, a phone
hotspot), or when someone wants to hide their address from the others, the two
sides meet at a **relay**: a small public program that passes bytes between
them. `inkwell-relay` is that program. One person of the group runs it, once.

> Status: the relay and the transport behind it (reconnection, hole punching,
> relay-only mode) are in place; the settings and status screens that expose
> them come with the sharing interface (stage 2, tasks 11 and 12).

## What a relay sees

- It sees **who** connects to it and **who talks to whom** (peer identities and
  network addresses), and how many bytes.
- It never sees **what** is said. Everything it carries is encrypted from end
  to end between the two people (TLS 1.3 over QUIC, Noise over TCP); the relay
  is not a party to that encryption.
- It stores nothing. Stopping it loses nothing: people reconnect when it is back.

## Run it

You need a machine with a public IP address (a small VPS is plenty) and the
port **7419**, UDP and TCP, open in its firewall.

```sh
inkwell-relay --key relay.key
```

or from a checkout of the project, `mise run relay:run` (key in
`bin/relay.key`; extra flags after `--`).

The binary is pure Go and static: `CGO_ENABLED=0 go build ./cmd/inkwell-relay`
gives one file to copy to the server, with no library to install.

### The key

The first run creates `relay.key`, readable by its owner only (mode `0600`).
It holds the relay's **identity**: the people who use the relay know it by that
key, so **keep it** (back it up, do not regenerate it) and **keep it secret**.
Anyone who has it can pass for your relay. It is never printed or logged.

### The address to share

`--print-addr` prints the addresses people put in their settings, one per line,
on standard output once the relay listens:

```sh
$ inkwell-relay --key relay.key --print-addr
/ip4/203.0.113.7/udp/7419/quic-v1/p2p/12D3KooW…
/ip4/203.0.113.7/tcp/7419/p2p/12D3KooW…
```

Share the line with the public IP address of your server (`/ip4/…` or
`/dns4/…` if you give it a name). The part after `/p2p/` is the relay's
identity and is what protects people from a fake relay at the same address.

### Flags

| Flag | Meaning |
|---|---|
| `--key FILE` | Key file; created if missing. Default `relay.key`. |
| `--listen MULTIADDR` | Address to listen on, repeatable. Default: QUIC and TCP on port 7419, IPv4 and IPv6. |
| `--print-addr` | Print the relay's addresses on standard output. |
| `--allow PEERID` | Serve only these peers, repeatable or comma-separated. Default: everyone. |

Examples:

```sh
inkwell-relay --key relay.key --listen /ip4/0.0.0.0/udp/443/quic-v1
inkwell-relay --key relay.key --allow 12D3KooWAAA…,12D3KooWBBB…
```

### As a service (systemd)

```ini
# /etc/systemd/system/inkwell-relay.service
[Unit]
Description=inkwell relay
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/inkwell-relay --key /var/lib/inkwell-relay/relay.key
StateDirectory=inkwell-relay
DynamicUser=yes
Restart=always
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes

[Install]
WantedBy=multi-user.target
```

## Limits

A relay is open to anyone who knows its address unless `--allow` is set, so it
is bounded:

| Limit | Value |
|---|---|
| Reservations (people who can be reached through it) | 256 |
| Simultaneous circuits per person | 16 |
| Reservations per IP address | 8 |
| Duration or volume of a circuit | unlimited |

libp2p's defaults cut a relayed connection after 2 minutes or 128 KiB, which
suits hole punching but not synchronising a document, so inkwell's relay lifts
that cut. If your relay is exposed and you do not want strangers to use your
bandwidth, set `--allow` with the identities of your group (shown in the app,
under "Détails techniques").

## How the app uses it

- It keeps a **reservation** on the relay, which is what lets the others reach
  it from any network. This is refreshed on its own and renewed within seconds
  of a network change.
- To reach someone it tries the addresses it last knew, then the address
  through the relay, which does not depend on where that person is now. This is
  why a laptop that wakes up on another network is found again within seconds.
- Once a relayed link is up, both sides try to open a direct one (hole
  punching). If it succeeds the traffic moves to it without a hiccup; the relay
  stays as a fallback.
- With **"Masquer mon adresse aux autres personnes"**, the app talks to the
  others through the relay only: it refuses every direct connection and does
  not hole punch. The relay still learns your address (it has to).

Until the settings screen exists, the app reads its relays from the
environment: `INKWELL_RELAYS` (addresses ending in `/p2p/<id>`, separated by
commas or spaces) and `INKWELL_NET=relay-only` (the "hide my address" mode). An
invalid combination, such as `relay-only` without a relay, stops the app at
start instead of silently falling back to direct connections.

## Check that it works

On another machine, with the relay's address `ADDR`:

1. Open inkwell on two computers on different networks, with `ADDR` in the
   relay setting of both.
2. Share a project from one and join it from the other. The status shows
   "relais" at first, then a direct link a few seconds later if the routers
   allow it.

To try everything on one machine, `mise run e2e:netsim` (`scripts/netsim`)
builds two home networks behind NATs and a public relay out of network
namespaces, with no root and no second computer, and checks hole punching
(cone NAT), the relayed fallback (symmetric NAT), a 60 s outage and a change
of network. It skips with the reason when the machine cannot (user namespaces
or nftables unavailable). `mise run e2e` also kills and restarts a relay under
two people typing, on the loopback interface.
