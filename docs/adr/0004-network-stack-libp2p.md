---
status: accepted
date: 2026-10-24
decision-makers: [Mathieu]
---

# Pile réseau maison : go-libp2p + wormhole-william derrière `Transport`

> Amendement proposé, en attente d’acceptation : [ADR 0032](0032-canal-invitation-wormhole.md).
> Elle sépare le rendez-vous Wormhole de `Transport` ; la décision historique ci-dessous
> reste acceptée tant que cet amendement ne l’est pas.

## Contexte et problème

Sans serveur, les pairs doivent se trouver (LAN, code court) et se connecter à travers les
NAT ; ~20–30 % des paires hors LAN passeront par un relais (mesure libp2p : 70 % ± 7,1 %
de succès du hole punching).

## Options envisagées

- go-libp2p (QUIC, Noise, mDNS, hole punching, relais circuit v2) + wormhole-william
- Iroh / Teamtype (Rust)
- tsnet (Tailscale)

## Décision

go-libp2p + wormhole-william, **derrière une interface Go minuscule** `Transport`
(`Send(ctx, peer, []byte) error`, `Recv(ctx) (Event, error)`, `Peers() []PeerID`,
`Local() PeerID`, `Close() error` ; signatures actuelles : `internal/transport/transport.go:101-118`). Un relais circuit v2 hébergé et
auto-hébergeable ; le LAN direct reste le chemin principal.

### Conséquences

- Bien : un seul binaire, licences MIT/Apache, `mocknet` pour les tests, tsnet en plan B
  derrière la même interface.
- Moins bien : colle réseau à écrire et à tester nous-mêmes.

### Confirmation

Aucun paquet hors `internal/transport/p2p` n'importe `github.com/libp2p/...`.
