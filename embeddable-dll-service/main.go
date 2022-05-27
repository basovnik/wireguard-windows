/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"net"

	"golang.org/x/sys/windows"

	"C"

	wgconn "golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/conf"
)

type TunnelHandle struct {
	device *device.Device
	uapi   net.Listener
}

var (
	tunnelHandle TunnelHandle
	log          *device.Logger
)

func init() {
	log = device.NewLogger(device.LogLevelVerbose, "")
}

//export wgTurnOnEmpty
func wgTurnOnEmpty() int32 {
	return 1
}

//export wgTurnOn
func wgTurnOn(interfaceNamePtr *uint16, settingsPtr *uint16) int32 {
	log.Verbosef("Starting %v %v", interfaceNamePtr, settingsPtr)

	interfaceName := windows.UTF16PtrToString(interfaceNamePtr)
	settings := windows.UTF16PtrToString(settingsPtr)

	tunDevice, err := tun.CreateTUN(interfaceName, 1420)
	if err != nil {
		log.Errorf("CreateTUN: %v", err)
		return -1
	} else {
		realInterfaceName, err2 := tunDevice.Name()
		if err2 == nil {
			interfaceName = realInterfaceName
		}
	}

	log.Verbosef("Creating interface instance")
	bind := wgconn.NewDefaultBind()
	dev := device.NewDevice(tunDevice, bind, log)

	log.Verbosef("Bringing peers up")
	err = dev.Up()
	if err != nil {
		log.Errorf("Up: %v", err)
		return -1
	}

	log.Verbosef("Setting interface configuration")
	config, err := conf.FromWgQuick(settings, interfaceName)
	if err != nil {
		log.Errorf("FromWgQuick: %v", err)
		return -1
	}
	uapi, err := ipc.UAPIListen(interfaceName)
	if err != nil {
		log.Errorf("UAPIListen: %v", err)
		return -1
	}
	err = dev.IpcSet(config.ToUAPI())
	if err != nil {
		log.Errorf("IpcSet: %v", err)
		return -1
	}

	log.Verbosef("Listening for UAPI requests")
	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				log.Verbosef("Accept: %v", err)
				continue
			}
			go dev.IpcHandle(conn)
		}
	}()

	tunnelHandle = TunnelHandle{device: dev, uapi: uapi}

	return 0
}

//export wgTurnOff
func wgTurnOff() {
	if tunnelHandle.uapi != nil {
		err := tunnelHandle.uapi.Close()
		if err != nil {
			log.Errorf("UAPI Close: %v", err)
		}
	}
	if tunnelHandle.device != nil {
		tunnelHandle.device.Close()
	}
}

func main() {}
