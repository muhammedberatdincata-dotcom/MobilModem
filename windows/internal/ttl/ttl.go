package ttl

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

type Manager struct {
	originalIPv4TTL int
	originalIPv6TTL int
	modified        bool
}

func NewManager() *Manager {
	return &Manager{
		originalIPv4TTL: 128,
		originalIPv6TTL: 128,
	}
}

func (m *Manager) ReadCurrentTTL() (int, int, error) {
	v4, err := m.readTTL("ipv4")
	if err != nil {
		return 0, 0, err
	}

	v6, err := m.readTTL("ipv6")
	if err != nil {
		return v4, 0, err
	}

	return v4, v6, nil
}

func (m *Manager) readTTL(version string) (int, error) {
	cmd := exec.Command("netsh", "int", version, "show", "glob")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to read %s ttl: %v", version, err)
	}

	re := regexp.MustCompile(`(?i)Default\s+(?:Cur)?Hop\s+Limit\s*:\s*(\d+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		return strconv.Atoi(matches[1])
	}

	return 128, nil
}

func (m *Manager) SetTTL(value int) error {
	cmdV4 := exec.Command("netsh", "int", "ipv4", "set", "glob", fmt.Sprintf("defaultcurhoplimit=%d", value))
	if out, err := cmdV4.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set ipv4 ttl: %s (%v)", string(out), err)
	}

	cmdV6 := exec.Command("netsh", "int", "ipv6", "set", "glob", fmt.Sprintf("defaultcurhoplimit=%d", value))
	if out, err := cmdV6.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set ipv6 ttl: %s (%v)", string(out), err)
	}

	return nil
}

func (m *Manager) Apply() error {
	v4, v6, err := m.ReadCurrentTTL()
	if err == nil {
		m.originalIPv4TTL = v4
		m.originalIPv6TTL = v6
	}

	if err := m.SetTTL(65); err != nil {
		return err
	}

	m.modified = true
	return nil
}

func (m *Manager) Restore() error {
	if !m.modified {
		return nil
	}

	exec.Command("netsh", "int", "ipv4", "set", "glob", fmt.Sprintf("defaultcurhoplimit=%d", m.originalIPv4TTL)).Run()
	exec.Command("netsh", "int", "ipv6", "set", "glob", fmt.Sprintf("defaultcurhoplimit=%d", m.originalIPv6TTL)).Run()

	m.modified = false
	return nil
}
