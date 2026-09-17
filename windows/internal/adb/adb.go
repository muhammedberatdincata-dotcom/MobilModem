package adb

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	embedpkg "github.com/mobilmodem/windows-client/internal/embed"
)

// Controller manages ADB device detection and port forwarding.
type Controller struct {
	adbPath    string
	forwarding bool
}

// NewController creates a new ADB controller.
func NewController() *Controller {
	return &Controller{}
}

// FindAdb locates adb.exe, checking extracted assets dir first, then app dir, then PATH.
func (c *Controller) FindAdb() (string, error) {
	// 1. Check extracted assets directory (embedded build)
	assetAdb := filepath.Join(embedpkg.GetAssetDir(), "adb.exe")
	if _, err := os.Stat(assetAdb); err == nil {
		c.adbPath = assetAdb
		return c.adbPath, nil
	}

	// 2. Check application directory
	exe, err := os.Executable()
	if err == nil {
		appDirAdb := filepath.Join(filepath.Dir(exe), "adb.exe")
		if _, err := os.Stat(appDirAdb); err == nil {
			c.adbPath = appDirAdb
			return c.adbPath, nil
		}
	}

	// 3. Check system PATH
	pathAdb, err := exec.LookPath("adb.exe")
	if err == nil {
		c.adbPath = pathAdb
		return c.adbPath, nil
	}

	return "", errors.New("adb.exe bulunamadı: uygulama dizininde veya PATH içinde mevcut değil")
}

// CheckDevice runs 'adb devices' and returns true if an authorized device is connected.
func (c *Controller) CheckDevice() (bool, error) {
	if c.adbPath == "" {
		if _, err := c.FindAdb(); err != nil {
			return false, err
		}
	}

	cmd := exec.Command(c.adbPath, "devices")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("adb devices başarısız: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "device") && !strings.Contains(line, "List of devices") {
			return true, nil
		}
	}

	return false, nil
}

// DeviceName returns the serial of the first connected device, or empty string.
func (c *Controller) DeviceName() string {
	if c.adbPath == "" {
		if _, err := c.FindAdb(); err != nil {
			return ""
		}
	}

	cmd := exec.Command(c.adbPath, "devices")
	out, _ := cmd.CombinedOutput()
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "device") && !strings.Contains(line, "List of devices") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				return parts[0]
			}
		}
	}
	return ""
}

// SetupForwarding creates an ADB port forward from local 10808 to device 10808.
func (c *Controller) SetupForwarding() error {
	if c.adbPath == "" {
		return errors.New("adb yolu ayarlanmamış")
	}

	cmd := exec.Command(c.adbPath, "forward", "tcp:10808", "tcp:10808")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb forward başarısız: %s (%v)", string(out), err)
	}

	c.forwarding = true
	return nil
}

// RemoveForwarding removes the ADB port forward rule.
func (c *Controller) RemoveForwarding() error {
	if c.adbPath == "" {
		return nil
	}

	cmd := exec.Command(c.adbPath, "forward", "--remove", "tcp:10808")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb forward --remove başarısız: %s (%v)", string(out), err)
	}

	c.forwarding = false
	return nil
}

// IsForwarding returns whether port forwarding is currently active.
func (c *Controller) IsForwarding() bool {
	return c.forwarding
}

// Cleanup removes forwarding if active. Safe to call multiple times.
func (c *Controller) Cleanup() {
	if c.forwarding {
		_ = c.RemoveForwarding()
	}
}
