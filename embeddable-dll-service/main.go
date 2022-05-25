/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"net"

	"C"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/conf"
)

type TunnelHandle struct {
	device *device.Device
	uapi   net.Listener
}

var tunnelHandles map[int32]TunnelHandle

func init() {
	tunnelHandles = make(map[int32]TunnelHandle)
}

//export wgTurnOn
func wgTurnOn(interfaceName string, settings string) int32 {
	log := device.NewLogger(device.LogLevelVerbose, "")

	tun, err := tun.CreateTUN(interfaceName, 1420)
	if err != nil {
		// unix.Close(int(tunFd))
		log.Errorf("CreateUnmonitoredTUNFromFD: %v", err)
		return -1
	}

	log.Verbosef("Creating interface instance")
	bind := conn.NewDefaultBind()
	dev := device.NewDevice(tun, bind, log)

	log.Verbosef("Setting interface configuration")
	config, err := conf.FromWgQuick(settings, interfaceName)
	if err != nil {
		log.Errorf("FromWgQuick: %v", err)
		return -1
	}
	uapi, err := ipc.UAPIListen(interfaceName)
	if err != nil {
		log.Errorf("FromWgQuick: %v", err)
		return -1
	}
	err = dev.IpcSet(config.ToUAPI())
	if err != nil {
		log.Errorf("FromWgQuick: %v", err)
		return -1
	}

	log.Verbosef("Bringing peers up")
	dev.Up()

	// var clamper mtuClamper
	// clamper = nativeTun
	// watcher.Configure(bind.(conn.BindSocketToInterface), clamper, nil, config, luid)

	log.Verbosef("Listening for UAPI requests")
	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				continue
			}
			go dev.IpcHandle(conn)
		}
	}()

	tunnelHandles[0] = TunnelHandle{device: dev, uapi: uapi}

	return 0
}

//export wgTurnOff
func wgTurnOff(tunnelHandle int32) {
	handle, ok := tunnelHandles[tunnelHandle]
	if !ok {
		return
	}
	delete(tunnelHandles, tunnelHandle)
	if handle.uapi != nil {
		handle.uapi.Close()
	}
	handle.device.Close()
}

func main() {}
