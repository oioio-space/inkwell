//go:build linux && !server

package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	gnomeManager = "org.gnome.SessionManager"
	gnomeClient  = gnomeManager + ".ClientPrivate"
)

func watchSessionLogout(ctx context.Context, closer *closeCoordinator, log *slog.Logger) {
	go func() {
		bus, err := dbus.ConnectSessionBus()
		if err != nil {
			log.Debug("session logout notification unavailable", "err", err)
			return
		}
		defer func() { _ = bus.Close() }()
		manager := bus.Object(gnomeManager, "/org/gnome/SessionManager")
		setup, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		var path dbus.ObjectPath
		if err := manager.CallWithContext(setup, gnomeManager+".RegisterClient", dbus.FlagNoAutoStart, appID, "").Store(&path); err != nil {
			log.Debug("GNOME session logout notification unavailable", "err", err)
			return
		}
		if err := bus.AddMatchSignalContext(setup, dbus.WithMatchSender(gnomeManager), dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(gnomeClient)); err != nil {
			log.Warn("subscribing to session logout", "err", err)
			return
		}
		client := bus.Object(gnomeManager, path)
		signals := make(chan *dbus.Signal, 8)
		bus.Signal(signals)
		defer bus.RemoveSignal(signals)
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signals:
				if !ok {
					return
				}
				if sig.Path != path {
					continue
				}
				switch sig.Name {
				case gnomeClient + ".QueryEndSession":
					respondToLogout(client, true, "", log)
				case gnomeClient + ".EndSession":
					end, cancel := context.WithTimeout(ctx, closeDeadline)
					saved := closer.waitExit(end)
					cancel()
					var reason string
					if !saved {
						log.Warn("session logout before saving could be confirmed")
						reason = "Les dernières modifications de ce projet n’ont pas pu être vérifiées."
					}
					respondToLogout(client, saved, reason, log)
				case gnomeClient + ".Stop":
					closer.requestAll()
				}
			}
		}
	}()
}

func respondToLogout(client dbus.BusObject, ok bool, reason string, log *slog.Logger) {
	reply, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := client.CallWithContext(reply, gnomeClient+".EndSessionResponse", 0, ok, reason).Err; err != nil {
		log.Debug("replying to session logout", "err", err)
	}
}
