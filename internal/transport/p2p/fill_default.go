//go:build !server

package p2p

import "github.com/libp2p/go-libp2p/core/network"

// The netsim filler (fill_server.go) is compiled into the headless end-to-end build only: here
// it does nothing, and the protocol it speaks does not exist.

func (n *Node) fillRegister()           {}
func (n *Node) fillAttach(network.Conn) {}
