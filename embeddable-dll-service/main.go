/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"log"
	"net"
	"os"

	"golang.org/x/sys/windows"

	"C"

	wgconn "golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/conf"
)

const logFilePath = "C:\\ProgramData\\JamfTrust\\logs\\wireguard-go.log"

type TunnelHandle struct {
	device *device.Device
	uapi   net.Listener
}

var (
	tunnelHandle TunnelHandle
	logFile      *os.File
	logger       *device.Logger
)

func init() {
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	logger = &device.Logger{
		Verbosef: log.New(logFile, "DEBUG:", log.Ldate|log.Ltime).Printf,
		Errorf:   log.New(logFile, "ERROR:", log.Ldate|log.Ltime).Printf,
	}
}

//export wgTurnOnEmpty
func wgTurnOnEmpty() int32 {
	return 1
}

//export wgTurnOn
func wgTurnOn(interfaceNamePtr *uint16, settingsPtr *uint16) int32 {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic: ", r)
		}
	}()

	logger.Verbosef("Starting %v %v", interfaceNamePtr, settingsPtr)

	interfaceName := windows.UTF16PtrToString(interfaceNamePtr)
	settings := windows.UTF16PtrToString(settingsPtr)

	tunDevice, err := tun.CreateTUN(interfaceName, 1420)
	if err != nil {
		logger.Errorf("CreateTUN: %v", err)
		logFile.Close()
		return -1
	} else {
		realInterfaceName, err2 := tunDevice.Name()
		if err2 == nil {
			interfaceName = realInterfaceName
		}
	}

	logger.Verbosef("Creating interface instance")
	bind := wgconn.NewDefaultBind()
	dev := device.NewDevice(tunDevice, bind, logger)

	logger.Verbosef("Bringing peers up")
	err = dev.Up()
	if err != nil {
		logger.Errorf("Up: %v", err)
		logFile.Close()
		return -1
	}

	logger.Verbosef("Setting interface configuration")
	config, err := conf.FromWgQuick(settings, interfaceName)
	if err != nil {
		logger.Errorf("FromWgQuick: %v", err)
		logFile.Close()
		return -1
	}
	uapi, err := ipc.UAPIListen(interfaceName)
	if err != nil {
		logger.Errorf("UAPIListen: %v", err)
		logFile.Close()
		return -1
	}
	err = dev.IpcSet(config.ToUAPI())
	if err != nil {
		logger.Errorf("IpcSet: %v", err)
		logFile.Close()
		return -1
	}

	logger.Verbosef("Listening for UAPI requests")
	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				logger.Verbosef("Accept: %v", err)
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
			logger.Errorf("UAPI Close: %v", err)
		}
	}
	if tunnelHandle.device != nil {
		tunnelHandle.device.Close()
	}

	if logFile != nil {
		logFile.Close()
	}
}

func main() {}
