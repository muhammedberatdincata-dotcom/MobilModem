package route

import (
	"fmt"
	"os/exec"
	"strings"
)

// Manager handles Windows IP routing table manipulation.
// Implements the two-halves routing trick (0.0.0.0/1 + 128.0.0.0/1) for clean
// traffic redirection without destroying the original default route.
type Manager struct {
	physicalGateway string
	routesAdded     bool
	proxyHost       string
	dnsSet          bool
}

// NewManager creates a new route manager.
func NewManager() *Manager {
	return &Manager{}
}

// DetectPhysicalGateway discovers the current default gateway from the route table.
func (m *Manager) DetectPhysicalGateway() error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric | Select-Object -First 1).NextHop")
	out, err := cmd.CombinedOutput()
	if err == nil {
		gw := strings.TrimSpace(string(out))
		if gw != "" && gw != "0.0.0.0" {
			m.physicalGateway = gw
			return nil
		}
	}

	// Fallback to route print parsing
	cmd2 := exec.Command("route", "print", "0.0.0.0")
	out2, err := cmd2.CombinedOutput()
	if err != nil {
		return fmt.Errorf("varsayılan ağ geçidi bulunamadı: %v", err)
	}

	lines := strings.Split(string(out2), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			m.physicalGateway = fields[2]
			return nil
		}
	}

	return fmt.Errorf("varsayılan ağ geçidi bulunamadı")
}

// GetPhysicalGateway returns the detected physical gateway IP.
func (m *Manager) GetPhysicalGateway() string {
	return m.physicalGateway
}

// Setup configures routing to tunnel all traffic through the Wintun adapter.
// proxyAddr format: "ip:port" (e.g. "127.0.0.1:10808" or "192.168.49.1:10808")
func (m *Manager) Setup(proxyAddr string) error {
	// Extract IP from proxyAddr (ip:port)
	parts := strings.Split(proxyAddr, ":")
	m.proxyHost = parts[0]

	if err := m.DetectPhysicalGateway(); err != nil {
		return err
	}

	// --- ROUTING LOOP PROTECTION ---
	// 1. Always add explicit route for localhost/loopback via physical gateway
	//    This prevents ADB USB communication from being captured by Wintun
	runRoute("add", "127.0.0.0", "mask", "255.0.0.0", "0.0.0.0", "metric", "1", "if", "1")

	// 2. Add host route for proxy server IP via physical gateway (unless localhost)
	if m.proxyHost != "127.0.0.1" && m.proxyHost != "localhost" {
		// Route for Wi-Fi Direct P2P subnet (192.168.49.0/24) via physical Wi-Fi adapter
		runRoute("add", "192.168.49.0", "mask", "255.255.255.0", m.proxyHost, "metric", "5")

		// Explicit host route for proxy IP via physical gateway
		cmd := exec.Command("route", "add", m.proxyHost, "mask", "255.255.255.255", m.physicalGateway, "metric", "5")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("proxy route eklenemedi: %s (%v)", string(out), err)
		}
	}

	// --- DNS LEAK PREVENTION ---
	runCmd("netsh", "interface", "ip", "set", "dns", "name=MobilModem", "static", "1.1.1.1")
	runCmd("netsh", "interface", "ip", "add", "dns", "name=MobilModem", "8.8.8.8", "index=2")
	m.dnsSet = true

	// --- SPLIT ROUTE (TWO-HALVES TRICK) ---
	// 0.0.0.0/1 and 128.0.0.0/1 are more specific than default 0.0.0.0/0,
	// so Windows routes all internet traffic through Wintun without deleting original route
	r1 := exec.Command("route", "add", "0.0.0.0", "mask", "128.0.0.0", "10.0.0.1", "metric", "5")
	if out, err := r1.CombinedOutput(); err != nil {
		return fmt.Errorf("0.0.0.0/1 route eklenemedi: %s (%v)", string(out), err)
	}

	r2 := exec.Command("route", "add", "128.0.0.0", "mask", "128.0.0.0", "10.0.0.1", "metric", "5")
	if out, err := r2.CombinedOutput(); err != nil {
		return fmt.Errorf("128.0.0.0/1 route eklenemedi: %s (%v)", string(out), err)
	}

	m.routesAdded = true
	return nil
}

// Cleanup removes all added routes and restores original routing table.
// Safe to call multiple times.
func (m *Manager) Cleanup() error {
	if !m.routesAdded {
		return nil
	}

	// Remove split routes
	runRoute("delete", "0.0.0.0", "mask", "128.0.0.0")
	runRoute("delete", "128.0.0.0", "mask", "128.0.0.0")

	// Remove proxy host route
	if m.proxyHost != "127.0.0.1" && m.proxyHost != "localhost" && m.proxyHost != "" {
		runRoute("delete", m.proxyHost, "mask", "255.255.255.255")
		runRoute("delete", "192.168.49.0", "mask", "255.255.255.0")
	}

	// Remove explicit loopback route (the system one remains)
	runRoute("delete", "127.0.0.0", "mask", "255.0.0.0", "0.0.0.0", "if", "1")

	m.routesAdded = false
	return nil
}

// IsActive returns whether routes are currently configured.
func (m *Manager) IsActive() bool {
	return m.routesAdded
}

// runRoute executes a route command silently, ignoring errors.
func runRoute(args ...string) {
	exec.Command("route", args...).Run()
}

// runCmd executes a command silently, ignoring errors.
func runCmd(name string, args ...string) {
	exec.Command(name, args...).Run()
}
