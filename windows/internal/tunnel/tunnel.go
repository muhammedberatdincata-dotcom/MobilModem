package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	embedpkg "github.com/mobilmodem/windows-client/internal/embed"
	"github.com/xjasonlyu/tun2socks/v2/engine"
	"golang.org/x/sys/windows"
)

// Manager coordinates the Wintun adapter and the in-process tun2socks engine.
type Manager struct {
	running   bool
	mode      string
	proxyAddr string
}

// NewManager creates a new tunnel manager.
func NewManager() *Manager {
	return &Manager{}
}

// prepareWintunDll ensures wintun.dll is available in the DLL search path.
func prepareWintunDll() {
	// 1. Check extracted temp directory
	assetDir := embedpkg.GetAssetDir()
	wintunInTemp := filepath.Join(assetDir, "wintun.dll")
	if _, err := os.Stat(wintunInTemp); err == nil {
		if ptr, err := syscall.UTF16PtrFromString(assetDir); err == nil {
			_ = windows.SetDllDirectory(ptr)
			return
		}
	}

	// 2. Check application directory
	if exe, err := os.Executable(); err == nil {
		appDir := filepath.Dir(exe)
		if ptr, err := syscall.UTF16PtrFromString(appDir); err == nil {
			_ = windows.SetDllDirectory(ptr)
		}
	}
}

// Start initializes the tun2socks engine and sets up the MobilModem Wintun adapter.
func (m *Manager) Start(mode string, proxyAddr string) error {
	m.mode = mode
	prepareWintunDll()

	var proxyURL string
	if mode == "usb" {
		m.proxyAddr = "127.0.0.1:10808"
		proxyURL = "socks5://" + m.proxyAddr
	} else {
		m.proxyAddr = proxyAddr
		proxyURL = "socks5://" + m.proxyAddr
	}

	key := &engine.Key{
		Device:   "tun://MobilModem",
		Proxy:    proxyURL,
		LogLevel: "info",
		MTU:      1500,
	}

	engine.Insert(key)

	if err := engine.Start(); err != nil {
		return fmt.Errorf("tun2socks motoru başlatılamadı: %v", err)
	}

	// Wait for the adapter to be created by tun2socks, then assign IP
	var lastErr error
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command("netsh", "interface", "ip", "set", "address",
			"name=MobilModem", "static", "10.0.0.2", "255.255.255.0", "none")
		out, cErr := cmd.CombinedOutput()
		if cErr == nil {
			m.running = true
			return nil
		}
		lastErr = fmt.Errorf("IP atama denemesi %d: %s (%v)", i+1, string(out), cErr)
	}

	engine.Stop()
	return fmt.Errorf("adaptör başlatılamadı (10 sn zaman aşımı): %v", lastErr)
}

// Stop terminates the tun2socks engine.
func (m *Manager) Stop() error {
	engine.Stop()
	m.running = false
	return nil
}

// IsRunning returns the current status of the tunnel engine.
func (m *Manager) IsRunning() bool {
	return m.running
}
