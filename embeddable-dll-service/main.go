/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"net"

	"golang.org/x/sys/windows"

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

//export wgTurnOnEmpty
func wgTurnOnEmpty() int32 {
	return 1
}

//export wgTurnOn
func wgTurnOn(interfaceNamePtr *uint16, settingsPtr *uint16) int32 {

	log := device.NewLogger(device.LogLevelVerbose, "")

	log.Verbosef("Starting %v %v", interfaceNamePtr, settingsPtr)

	interfaceName := windows.UTF16PtrToString(interfaceNamePtr)
	settings := windows.UTF16PtrToString(settingsPtr)

	tun, err := tun.CreateTUN(interfaceName, 1420)
	if err != nil {
		// unix.Close(int(tunFd))
		log.Errorf("CreateUnmonitoredTUNFromFD: %v", err)
		return -1
	} else {
		realInterfaceName, err2 := tun.Name()
		if err2 == nil {
			interfaceName = realInterfaceName
		}
	}

	log.Verbosef("Creating interface instance")
	bind := conn.NewDefaultBind()
	dev := device.NewDevice(tun, bind, log)

	log.Verbosef("Bringing peers up")
	dev.Up()

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

	idx := int32(0)
	tunnelHandles[idx] = TunnelHandle{device: dev, uapi: uapi}

	return idx
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
