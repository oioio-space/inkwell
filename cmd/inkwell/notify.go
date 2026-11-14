package main

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// desktopNotify shows m through the desktop's notification service on the session bus
// (org.freedesktop.Notifications). It fails when there is no bus or no service.
func desktopNotify(ctx context.Context, m message) error {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	return obj.CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
		"Inkwell", uint32(0), appID, m.title, m.body, []string{}, map[string]dbus.Variant{}, int32(-1)).Err
}
