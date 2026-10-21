package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"mitmAny/internal/android"
)

type AndroidState struct {
	Listening bool     `json:"listening"`
	Enabled   bool     `json:"enabled"`
	Serial    string   `json:"serial"`
	Address   string   `json:"address"`
	Pending   []string `json:"pending"`
}

func (a *App) androidStateLocked() AndroidState {
	s := AndroidState{Enabled: a.mobileEngine != nil && a.mobileSerial != "", Serial: a.mobileSerial, Listening: a.mobileEngine != nil, Address: android.Proxy, Pending: []string{}}
	if a.android != nil {
		s.Pending = a.android.Pending()
	}
	return s
}
func (a *App) GetAndroidState() AndroidState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.androidStateLocked()
}
func (a *App) AndroidDevices() ([]android.Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.android == nil {
		return nil, errors.New(a.initError)
	}
	return a.android.Devices()
}
func (a *App) SetAndroidListening(enabled bool) (AndroidState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.setAndroidListeningLocked(enabled)
}
func (a *App) setAndroidListeningLocked(enabled bool) (AndroidState, error) {
	if a.android == nil || a.certificates == nil || a.observer == nil {
		return a.androidStateLocked(), errors.New(a.initError)
	}
	if !enabled {
		if err := a.stopAndroidLocked(); err != nil {
			return a.androidStateLocked(), err
		}
		return a.androidStateLocked(), nil
	}
	if a.mobileEngine != nil {
		return a.androidStateLocked(), nil
	}
	if !a.certificates.Status().Valid {
		return a.androidStateLocked(), errors.New("CA 证书已过期或无效")
	}
	listener, err := net.Listen("tcp", android.Proxy)
	if err != nil {
		return a.androidStateLocked(), fmt.Errorf("手机代理端口 9081 不可用: %w", err)
	}
	engine, err := proxy.NewProxy(&proxy.Options{Addr: android.Proxy, ReadOnlyCapture: true, NewCaFunc: func() (cert.CA, error) { return a.certificates.CA, nil }})
	if err != nil {
		listener.Close()
		return a.androidStateLocked(), err
	}
	engine.SetUpstreamProxy(func(*http.Request) (*url.URL, error) { return nil, nil })
	engine.SetShouldInterceptRule(func(*http.Request) bool { return true })
	engine.AddAddon(proxy.NewUpstreamCertAddon(true))
	engine.AddAddon(a.observer)
	a.mobileEngine = engine
	a.mobileListener = listener
	a.observer.SetRecording(a.recording)
	go func() {
		serveErr := engine.Serve(listener)
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.mobileEngine != engine {
			return
		}
		a.mobileEngine = nil
		a.mobileSerial = ""
		a.mobileListener = nil
		a.observer.SetRecording(a.recording && a.running)
		if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) && !errors.Is(serveErr, http.ErrServerClosed) {
			a.lastError = "手机代理监听中断: " + serveErr.Error()
		}
	}()
	return a.androidStateLocked(), nil
}

// SetAndroidEnabled treats the listener, ADB tunnel, and device proxy as one operation.
// A failed enable never reports an active proxy; failed restoration keeps serving.
func (a *App) SetAndroidEnabled(serial string, enabled bool) (AndroidState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !enabled {
		err := a.stopAndroidLocked()
		return a.androidStateLocked(), err
	}
	if a.android == nil || a.certificates == nil || a.observer == nil {
		return a.androidStateLocked(), errors.New("Android 代理尚未初始化")
	}
	if a.mobileSerial != "" {
		if a.mobileSerial == serial && a.mobileEngine != nil {
			return a.androidStateLocked(), nil
		}
		return a.androidStateLocked(), errors.New("请先关闭当前手机代理，再切换设备")
	}
	if len(a.android.Pending()) > 0 {
		return a.androidStateLocked(), errors.New("请先关闭开关，恢复上次的手机代理")
	}
	if serial == "" {
		return a.androidStateLocked(), errors.New("请先连接并选择手机")
	}
	devices, err := a.android.Devices()
	if err != nil {
		return a.androidStateLocked(), err
	}
	ready := false
	for _, device := range devices {
		if device.Serial == serial && device.State == "device" {
			ready = true
			break
		}
	}
	if !ready {
		return a.androidStateLocked(), errors.New("请连接手机并允许 USB 调试后重试")
	}
	hadListener := a.mobileEngine != nil
	if _, err = a.setAndroidListeningLocked(true); err != nil {
		return a.androidStateLocked(), err
	}
	if err = a.android.Enable(serial); err != nil {
		// If rollback could not restore the phone, retain the listener and backup.
		if !hadListener {
			err = errors.Join(err, a.stopAndroidLocked())
		}
		return a.androidStateLocked(), err
	}
	a.mobileSerial = serial
	return a.androidStateLocked(), nil
}

func (a *App) SetAndroidProxy(serial string) (AndroidState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.android == nil || a.mobileEngine == nil {
		return a.androidStateLocked(), errors.New("请先打开手机代理监听")
	}
	err := a.android.Enable(serial)
	if err == nil {
		a.mobileSerial = serial
	}
	return a.androidStateLocked(), err
}
func (a *App) RestoreAndroidProxy(serial string) (AndroidState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.android == nil {
		return a.androidStateLocked(), errors.New(a.initError)
	}
	err := a.android.Restore(serial)
	if err == nil && a.mobileSerial == serial {
		a.mobileSerial = ""
	}
	return a.androidStateLocked(), err
}
func (a *App) InstallAndroidCertificate(serial string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.android == nil || a.certificates == nil {
		return "", errors.New(a.initError)
	}
	dir, err := os.MkdirTemp("", "mitmany-ca-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "mitmAny-ca.crt")
	if err = a.certificates.Export(path); err != nil {
		return "", err
	}
	return a.android.InstallCertificate(serial, path)
}
func (a *App) stopAndroidLocked() error {
	if a.android != nil {
		if err := a.android.RestoreAll(); err != nil {
			return err
		}
	}
	a.mobileSerial = ""
	if a.mobileEngine != nil {
		engine := a.mobileEngine
		a.mobileEngine = nil
		if a.mobileListener != nil {
			a.mobileListener.Close()
			a.mobileListener = nil
		}
		engine.Close()
	}
	if a.observer != nil {
		a.observer.SetRecording(a.recording && a.running)
	}
	return nil
}
