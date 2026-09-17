package gui

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/mobilmodem/windows-client/internal/adb"
	"github.com/mobilmodem/windows-client/internal/route"
	"github.com/mobilmodem/windows-client/internal/ttl"
	"github.com/mobilmodem/windows-client/internal/tunnel"
)

// App encapsulates the Fyne UI and network managers for MobilModem.
type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window

	adbCtrl   *adb.Controller
	ttlMgr    *ttl.Manager
	tunnelMgr *tunnel.Manager
	routeMgr  *route.Manager

	// UI Components
	modeSelect   *widget.RadioGroup
	targetIP     *widget.Entry
	ipCard       *fyne.Container
	btnAction    *widget.Button
	statusLabel  *widget.Label

	// Indicator badges
	badgeADB    *widget.Label
	badgeWintun *widget.Label
	badgeTTL    *widget.Label

	// Live stats
	pingLabel  *widget.Label
	speedLabel *widget.Label

	// Log container & toggle
	logBox       *widget.Entry
	logContainer *fyne.Container
	btnToggleLog *widget.Button
	isLogVisible bool

	// State
	isConnecting bool
	statStopChan chan struct{}
	mu           sync.Mutex
}

// NewApp instantiates the MobilModem desktop UI.
func NewApp() *App {
	a := &App{
		fyneApp:   app.NewWithID("com.mobilmodem.client"),
		adbCtrl:   adb.NewController(),
		ttlMgr:    ttl.NewManager(),
		tunnelMgr: tunnel.NewManager(),
		routeMgr:  route.NewManager(),
	}

	a.fyneApp.Settings().SetTheme(theme.DarkTheme())

	a.mainWindow = a.fyneApp.NewWindow("MobilModem — Operatör Korumalı L3 Ağ Tüneli")
	a.mainWindow.Resize(fyne.NewSize(520, 680))
	a.mainWindow.SetFixedSize(false)

	// Intercept window close for graceful cleanup
	a.mainWindow.SetCloseIntercept(func() {
		a.GracefulShutdown()
		a.mainWindow.Close()
	})

	return a
}

// Log writes timestamped log messages to the UI entry widget.
func (a *App) Log(msg string) {
	ts := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s\n", ts, msg)
	if a.logBox != nil {
		a.logBox.SetText(a.logBox.Text + entry)
		a.logBox.CursorRow = len(a.logBox.Text)
		a.logBox.Refresh()
	}
}

func (a *App) setupUI() {
	// 1. Header Banner
	headerTitle := widget.NewLabelWithStyle("⚡ MobilModem v1.0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	headerSubtitle := widget.NewLabelWithStyle("Mobil Operatör Kısıtlamasız Tam L3 Tünel Adaptörü", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	headerBox := container.NewVBox(headerTitle, headerSubtitle)

	// 2. Mode Selection Card
	a.modeSelect = widget.NewRadioGroup([]string{
		"⚡ USB Kablo Modu (Ultra Hız)",
		"📶 Wi-Fi Direct Modu (P2P)",
	}, func(s string) {
		if strings.Contains(s, "Wi-Fi Direct") {
			a.ipCard.Show()
			a.badgeADB.SetText("ADB: Devre Dışı (Wi-Fi)")
		} else {
			a.ipCard.Hide()
			a.refreshADBStatus()
		}
	})
	a.modeSelect.Horizontal = false
	a.modeSelect.SetSelected("⚡ USB Kablo Modu (Ultra Hız)")

	a.targetIP = widget.NewEntry()
	a.targetIP.SetText("192.168.49.1")
	a.targetIP.SetPlaceHolder("Telefon P2P IP Adresi (Örn: 192.168.49.1)")

	a.ipCard = container.NewVBox(
		widget.NewLabelWithStyle("Hedef Ağ Geçidi IP:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		a.targetIP,
	)
	a.ipCard.Hide()

	modeCard := widget.NewCard("Bağlantı Türü", "", container.NewVBox(
		a.modeSelect,
		a.ipCard,
	))

	// 3. System Status Card
	a.badgeADB = widget.NewLabel("ADB Cihazı: ⏳ Bekleniyor...")
	a.badgeWintun = widget.NewLabel("Wintun Sanal Kart: ❌ Pasif")
	a.badgeTTL = widget.NewLabel("TTL Koruması: ❌ Kapalı (Varsayılan 128)")

	statusCard := widget.NewCard("Sistem Güvenlik ve Donanım Durumu", "", container.NewVBox(
		a.badgeADB,
		a.badgeWintun,
		a.badgeTTL,
	))

	// 4. Live Stats Card (Ping & Hız)
	a.pingLabel = widget.NewLabelWithStyle("Ping: -- ms", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	a.speedLabel = widget.NewLabelWithStyle("Durum: Bağlantı Yok", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	statsGrid := container.NewGridWithColumns(2,
		widget.NewCard("", "", a.pingLabel),
		widget.NewCard("", "", a.speedLabel),
	)

	// 5. Large Action Button
	a.statusLabel = widget.NewLabelWithStyle("Bağlantı Hazır", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	a.btnAction = widget.NewButton("TÜNELİ BAŞLAT", a.onToggleClicked)
	a.btnAction.Importance = widget.HighImportance

	actionContainer := container.NewVBox(
		a.btnAction,
		a.statusLabel,
	)

	// 6. Collapsible Log Window
	a.logBox = widget.NewMultiLineEntry()
	a.logBox.Disable()
	a.logBox.SetMinRowsVisible(6)

	a.logContainer = container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Ayrıntılı Olay Günlüğü", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		a.logBox,
	)
	a.logContainer.Hide()
	a.isLogVisible = false

	a.btnToggleLog = widget.NewButton("▼ Log Penceresini Göster", func() {
		a.isLogVisible = !a.isLogVisible
		if a.isLogVisible {
			a.logContainer.Show()
			a.btnToggleLog.SetText("▲ Log Penceresini Gizle")
		} else {
			a.logContainer.Hide()
			a.btnToggleLog.SetText("▼ Log Penceresini Göster")
		}
	})

	// Master Layout
	mainContent := container.NewVBox(
		headerBox,
		widget.NewSeparator(),
		modeCard,
		statusCard,
		statsGrid,
		widget.NewSeparator(),
		actionContainer,
		a.btnToggleLog,
		a.logContainer,
	)

	scrollable := container.NewVScroll(mainContent)
	a.mainWindow.SetContent(scrollable)

	// Initial ADB background check
	go a.periodicADBCheck()
}

func (a *App) refreshADBStatus() {
	if strings.Contains(a.modeSelect.Selected, "Wi-Fi") {
		return
	}
	dev, err := a.adbCtrl.CheckDevice()
	if err == nil && dev {
		serial := a.adbCtrl.DeviceName()
		if serial != "" {
			a.badgeADB.SetText(fmt.Sprintf("ADB Cihazı: ✅ Bağlı (%s)", serial))
		} else {
			a.badgeADB.SetText("ADB Cihazı: ✅ Bağlı")
		}
	} else {
		a.badgeADB.SetText("ADB Cihazı: ❌ Cihaz Bekleniyor (USB Debugging Açın)")
	}
}

func (a *App) periodicADBCheck() {
	for {
		time.Sleep(3 * time.Second)
		if !a.tunnelMgr.IsRunning() && !strings.Contains(a.modeSelect.Selected, "Wi-Fi") {
			a.refreshADBStatus()
		}
	}
}

func (a *App) onToggleClicked() {
	if a.isConnecting {
		return
	}

	if a.tunnelMgr.IsRunning() {
		go a.Disconnect()
	} else {
		go a.Connect()
	}
}

// Connect establishes ADB forward (if USB), sets TTL=65, spins Wintun & routes.
func (a *App) Connect() {
	a.mu.Lock()
	a.isConnecting = true
	a.btnAction.Disable()
	a.btnAction.SetText("BAĞLANIYOR...")
	a.statusLabel.SetText("Tünel bileşenleri başlatılıyor...")
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isConnecting = false
		a.btnAction.Enable()
		a.mu.Unlock()
	}()

	isUSB := strings.Contains(a.modeSelect.Selected, "USB")
	proxyAddr := "127.0.0.1:10808"
	mode := "usb"

	a.Log("Bağlantı süreci başlatıldı.")

	if isUSB {
		a.Log("ADB aygıtı denetleniyor...")
		ok, err := a.adbCtrl.CheckDevice()
		if err != nil || !ok {
			a.Log("HATA: USB ile bağlı Android cihaz bulunamadı! Lütfen USB Hata Ayıklamayı açın.")
			a.statusLabel.SetText("HATA: Android cihaz algılanamadı!")
			a.btnAction.SetText("TÜNELİ BAŞLAT")
			return
		}
		a.badgeADB.SetText("ADB Cihazı: ✅ Bağlı ve Yetkili")

		a.Log("ADB port yönlendirmesi kuruluyor (tcp:10808 -> tcp:10808)...")
		if err := a.adbCtrl.SetupForwarding(); err != nil {
			a.Log("HATA: ADB port yönlendirme hatası: " + err.Error())
			a.statusLabel.SetText("HATA: ADB Port Yönlendirme Başarısız")
			a.btnAction.SetText("TÜNELİ BAŞLAT")
			return
		}
	} else {
		mode = "p2p"
		target := strings.TrimSpace(a.targetIP.Text)
		if target == "" {
			target = "192.168.49.1"
		}
		proxyAddr = target + ":10808"
		a.Log(fmt.Sprintf("Wi-Fi Direct modu seçildi. Hedef Gateway: %s", proxyAddr))
	}

	// 1. Set TTL = 65 (Operator Tethering Bypass)
	a.Log("Operatör Hotspot/Tethering tespiti engelleniyor: TTL = 65 yapılıyor...")
	if err := a.ttlMgr.Apply(); err != nil {
		a.Log("UYARI: TTL 65 uygulanamadı: " + err.Error())
	} else {
		a.badgeTTL.SetText("TTL Koruması: ✅ TTL=65 Aktif (Hotspot Bypass)")
		a.Log("TTL başarıyla 65 olarak ayarlandı. Çıkış paketleri telefondan geçerken 64 olacak.")
	}

	// 2. Start Wintun & in-process tun2socks engine
	a.Log("Wintun sanal ağ adaptörü ve gVisor netstack başlatılıyor...")
	if err := a.tunnelMgr.Start(mode, proxyAddr); err != nil {
		a.Log("HATA: Wintun adaptörü başlatılamadı: " + err.Error())
		a.statusLabel.SetText("HATA: Wintun Adaptörü Başarısız")
		a.Disconnect()
		return
	}
	a.badgeWintun.SetText("Wintun Sanal Kart: ✅ Aktif (10.0.0.2)")

	// 3. Configure Route Table (Two-Halves Route with Loopback & P2P Exclusions)
	a.Log("Windows IP Yönlendirme tablosu yapılandırılıyor (0.0.0.0/1 ve 128.0.0.0/1)...")
	if err := a.routeMgr.Setup(proxyAddr); err != nil {
		a.Log("HATA: Route tablosu uygulanamadı: " + err.Error())
		a.statusLabel.SetText("HATA: Ağ Rotaları Eklenemedi")
		a.Disconnect()
		return
	}

	a.Log("✅ TÜM İNTERNET BAĞLANTISI BAŞARIYLA TÜNELLENDİ!")
	a.statusLabel.SetText("🟢 BAĞLI: Mobil Operatör Kotası Harcanmıyor")
	a.btnAction.SetText("BAĞLANTIYI KES")
	a.btnAction.Importance = widget.DangerImportance

	// Start Ping & Stats loop
	a.startLiveStats()
}

// Disconnect gracefully rolls back routes, stops tun2socks, restores TTL, removes adb forward.
func (a *App) Disconnect() {
	a.mu.Lock()
	a.stopLiveStats()
	a.btnAction.Disable()
	a.btnAction.SetText("KOPARILIYOR...")
	a.statusLabel.SetText("Ayarlar orijinal haline döndürülüyor...")
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.btnAction.SetText("TÜNELİ BAŞLAT")
		a.btnAction.Importance = widget.HighImportance
		a.btnAction.Enable()
		a.statusLabel.SetText("Bağlantı Kesildi")
		a.badgeWintun.SetText("Wintun Sanal Kart: ❌ Pasif")
		a.badgeTTL.SetText("TTL Koruması: ❌ Kapalı (Varsayılan 128)")
		a.pingLabel.SetText("Ping: -- ms")
		a.speedLabel.SetText("Durum: Bağlantı Yok")
		a.refreshADBStatus()
		a.mu.Unlock()
	}()

	a.Log("Yönlendirme kuralları kaldırılıyor...")
	_ = a.routeMgr.Cleanup()

	a.Log("Wintun adaptörü ve tun2socks kapatılıyor...")
	_ = a.tunnelMgr.Stop()

	a.Log("TTL değeri orijinal haline (128) geri getiriliyor...")
	_ = a.ttlMgr.Restore()

	a.Log("ADB yönlendirme kuralları temizleniyor...")
	a.adbCtrl.Cleanup()

	a.Log("Temizlik tamamlandı. Sistem ağ durumu tamamen eski haline döndü.")
}

// GracefulShutdown ensures all routes and adapters are cleaned if application exits.
func (a *App) GracefulShutdown() {
	a.stopLiveStats()
	_ = a.routeMgr.Cleanup()
	_ = a.tunnelMgr.Stop()
	_ = a.ttlMgr.Restore()
	a.adbCtrl.Cleanup()
}

func (a *App) startLiveStats() {
	a.statStopChan = make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.statStopChan:
				return
			case <-ticker.C:
				if !a.tunnelMgr.IsRunning() {
					return
				}
				// Measure TCP handshake latency to DNS
				start := time.Now()
				conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 1500*time.Millisecond)
				if err == nil {
					latency := time.Since(start).Milliseconds()
					conn.Close()
					a.pingLabel.SetText(fmt.Sprintf("Ping: %d ms (1.1.1.1)", latency))
					a.speedLabel.SetText("Tünel: 🟢 Aktif & Akıyor")
				} else {
					a.pingLabel.SetText("Ping: Zaman Aşımı")
					a.speedLabel.SetText("Tünel: ⚠️ Yanıt Yok")
				}
			}
		}
	}()
}

func (a *App) stopLiveStats() {
	if a.statStopChan != nil {
		close(a.statStopChan)
		a.statStopChan = nil
	}
}

// Run displays the window and runs the event loop.
func (a *App) Run() {
	a.setupUI()
	a.mainWindow.ShowAndRun()
}
