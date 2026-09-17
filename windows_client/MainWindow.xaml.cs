using System;
using System.Diagnostics;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Media;
using System.Windows.Threading;

namespace MobilModemClient;

public partial class MainWindow : Window
{
    private readonly TtlManager _ttlManager = new();
    private readonly RouteManager _routeManager = new();
    private readonly AdbManager _adbManager = new();
    private readonly WintunManager _wintunManager = new();
    private readonly ApkManager _apkManager = new();

    private DispatcherTimer? _adbPollTimer;
    private DispatcherTimer? _statsTimer;
    private bool _isConnecting;
    private bool _isLogExpanded;

    public MainWindow()
    {
        InitializeComponent();
        InitBackgroundTimers();
        Log("MobilModem İstemcisi başlatıldı.");
    }

    private void InitBackgroundTimers()
    {
        // Periodic ADB device check every 3s
        _adbPollTimer = new DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(3)
        };
        _adbPollTimer.Tick += (s, e) => CheckAdbDeviceStatus();
        _adbPollTimer.Start();

        // Check immediately
        CheckAdbDeviceStatus();

        // Live stats timer (ping) every 2s
        _statsTimer = new DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(2)
        };
        _statsTimer.Tick += async (s, e) => await MeasureLivePingAsync();
    }

    private void Log(string message)
    {
        Dispatcher.Invoke(() =>
        {
            string ts = DateTime.Now.ToString("HH:mm:ss");
            TxtLogs.AppendText($"[{ts}] {message}\n");
            TxtLogs.ScrollToEnd();
        });
    }

    private void Mode_Changed(object sender, RoutedEventArgs e)
    {
        if (P2pSettingsBox == null) return;

        bool isP2p = RbP2pMode.IsChecked == true;
        P2pSettingsBox.Visibility = isP2p ? Visibility.Visible : Visibility.Collapsed;

        if (isP2p)
        {
            LblAdbStatus.Text = "ADB Cihazı: Devre Dışı (Wi-Fi Direct Seçildi)";
            LblAdbStatus.Foreground = new SolidColorBrush(Color.FromRgb(161, 161, 170));
        }
        else
        {
            CheckAdbDeviceStatus();
        }
    }

    private void CheckAdbDeviceStatus()
    {
        if (_wintunManager.IsRunning || RbP2pMode.IsChecked == true) return;

        var (connected, serial) = _adbManager.CheckDevice();
        if (connected)
        {
            LblAdbStatus.Text = $"ADB Cihazı: ✅ Bağlı ({serial})";
            LblAdbStatus.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
        }
        else
        {
            LblAdbStatus.Text = "ADB Cihazı: ❌ Bekleniyor (USB Debugging Açın)";
            LblAdbStatus.Foreground = new SolidColorBrush(Color.FromRgb(244, 63, 94));
        }
    }

    private async void BtnAction_Click(object sender, RoutedEventArgs e)
    {
        if (_isConnecting) return;

        if (_wintunManager.IsRunning)
        {
            await DisconnectTunnelAsync();
        }
        else
        {
            await ConnectTunnelAsync();
        }
    }

    private async Task ConnectTunnelAsync()
    {
        _isConnecting = true;
        BtnAction.IsEnabled = false;
        BtnAction.Content = "BAĞLANIYOR...";
        LblStatusDesc.Text = "Tünel bileşenleri hazırlanıyor...";

        bool isUsb = RbUsbMode.IsChecked == true;
        string mode = isUsb ? "usb" : "p2p";
        string targetIp = TxtTargetIp.Text.Trim();
        if (string.IsNullOrEmpty(targetIp)) targetIp = "192.168.49.1";
        string proxyAddress = isUsb ? "127.0.0.1:10808" : $"{targetIp}:10808";

        Log($"Bağlantı başlatılıyor. Mod: {(isUsb ? "USB Kablo" : "Wi-Fi Direct")}");

        await Task.Run(() =>
        {
            // 1. ADB Forward if USB
            if (isUsb)
            {
                Log("ADB cihazı doğrulanıyor...");
                var (connected, _) = _adbManager.CheckDevice();
                if (!connected)
                {
                    Log("HATA: USB ile bağlı Android cihaz bulunamadı!");
                    Dispatcher.Invoke(() =>
                    {
                        LblStatusDesc.Text = "HATA: Android cihaz bulunamadı!";
                        BtnAction.Content = "TÜNELİ BAŞLAT";
                        BtnAction.IsEnabled = true;
                        _isConnecting = false;
                    });
                    return;
                }

                Log("ADB port yönlendirmesi kuruluyor (tcp:10808 -> tcp:10808)...");
                if (!_adbManager.SetupForwarding(out string? fwdErr))
                {
                    Log($"HATA: ADB port yönlendirme başarısız: {fwdErr}");
                    Dispatcher.Invoke(() =>
                    {
                        LblStatusDesc.Text = "HATA: ADB Yönlendirme Başarısız";
                        BtnAction.Content = "TÜNELİ BAŞLAT";
                        BtnAction.IsEnabled = true;
                        _isConnecting = false;
                    });
                    return;
                }
            }

            // 2. TTL = 65 (Operator Hotspot Bypass)
            Log("Operatör Hotspot tespiti engelleniyor: TTL=65 ayarlanıyor...");
            if (_ttlManager.ApplyTtl65(out string? ttlErr))
            {
                Log("TTL=65 başarıyla uygulandı.");
                Dispatcher.Invoke(() =>
                {
                    LblTtlStatus.Text = "TTL Koruması: ✅ TTL=65 Aktif (Operatör Korumalı)";
                    LblTtlStatus.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
                });
            }
            else
            {
                Log($"UYARI: TTL uygulanamadı: {ttlErr}");
            }

            // 3. Start Wintun & tun2socks
            Log("Wintun sanal adaptörü (MobilModem) başlatılıyor...");
            if (!_wintunManager.StartTunnel(mode, proxyAddress, out string? tunErr))
            {
                Log($"HATA: Wintun tüneli başlatılamadı: {tunErr}");
                Dispatcher.Invoke(() =>
                {
                    LblStatusDesc.Text = "HATA: Wintun Başlatılamadı";
                    Rollback();
                    BtnAction.Content = "TÜNELİ BAŞLAT";
                    BtnAction.IsEnabled = true;
                    _isConnecting = false;
                });
                return;
            }

            Dispatcher.Invoke(() =>
            {
                LblWintunStatus.Text = "Wintun Sanal Kart: ✅ Aktif (10.0.0.2)";
                LblWintunStatus.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
            });

            // 4. Configure Split Route
            Log("Windows yönlendirme tablosu yapılandırılıyor (0.0.0.0/1 ve 128.0.0.0/1)...");
            if (!_routeManager.SetupRoutes(proxyAddress, out string? routeErr))
            {
                Log($"HATA: Yönlendirme kuralları uygulanamadı: {routeErr}");
                Dispatcher.Invoke(() =>
                {
                    LblStatusDesc.Text = "HATA: Rota Tablosu Başarısız";
                    Rollback();
                    BtnAction.Content = "TÜNELİ BAŞLAT";
                    BtnAction.IsEnabled = true;
                    _isConnecting = false;
                });
                return;
            }

            Log("✅ BAĞLANTI TAMAMLANDI! Tüm internet trafiği hücresel ağ üzerinden tünelleniyor.");

            Dispatcher.Invoke(() =>
            {
                BtnAction.Content = "BAĞLANTIYI KES";
                BtnAction.Background = new SolidColorBrush(Color.FromRgb(244, 63, 94));
                BtnAction.IsEnabled = true;
                LblStatusDesc.Text = "🟢 BAĞLI: Mobil Operatör Hotspot Kotası Etkilenmiyor";
                LblTunnelFlow.Text = "Aktif & Akıyor";
                LblTunnelFlow.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
                _isConnecting = false;
                _statsTimer?.Start();
            });
        });
    }

    private async Task DisconnectTunnelAsync()
    {
        BtnAction.IsEnabled = false;
        BtnAction.Content = "KOPARILIYOR...";
        LblStatusDesc.Text = "Ayarlar orijinal haline getiriliyor...";

        await Task.Run(() => Rollback());

        BtnAction.Content = "TÜNELİ BAŞLAT";
        BtnAction.Background = new SolidColorBrush(Color.FromRgb(16, 185, 129));
        BtnAction.IsEnabled = true;
        LblStatusDesc.Text = "Bağlantı Kurulmaya Hazır";
        LblWintunStatus.Text = "Wintun Sanal Kart: ❌ Pasif";
        LblWintunStatus.Foreground = new SolidColorBrush(Color.FromRgb(228, 228, 231));
        LblTtlStatus.Text = "TTL Koruması: ❌ Kapalı (Varsayılan 128)";
        LblTtlStatus.Foreground = new SolidColorBrush(Color.FromRgb(228, 228, 231));
        LblPing.Text = "-- ms";
        LblTunnelFlow.Text = "Bağlantı Yok";
        LblTunnelFlow.Foreground = new SolidColorBrush(Color.FromRgb(244, 63, 94));
        _statsTimer?.Stop();
        CheckAdbDeviceStatus();
    }

    private void Rollback()
    {
        Log("Ağ yönlendirme kuralları temizleniyor...");
        _routeManager.Cleanup();

        Log("Wintun sanal adaptörü kapatılıyor...");
        _wintunManager.StopTunnel();

        Log("TTL değeri orijinal haline (128) getiriliyor...");
        _ttlManager.RestoreOriginal();

        Log("ADB yönlendirmesi temizleniyor...");
        _adbManager.RemoveForwarding();

        Log("Tüm ayarlar başarıyla sıfırlandı.");
    }

    private async Task MeasureLivePingAsync()
    {
        if (!_wintunManager.IsRunning) return;

        try
        {
            var sw = Stopwatch.StartNew();
            using var client = new TcpClient();
            var connectTask = client.ConnectAsync("1.1.1.1", 53);
            if (await Task.WhenAny(connectTask, Task.Delay(1200)) == connectTask)
            {
                sw.Stop();
                LblPing.Text = $"{sw.ElapsedMilliseconds} ms";
                LblTunnelFlow.Text = "Aktif & Akıyor";
                LblTunnelFlow.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
            }
            else
            {
                LblPing.Text = "Zaman Aşımı";
                LblTunnelFlow.Text = "⚠️ Yanıt Yok";
                LblTunnelFlow.Foreground = new SolidColorBrush(Color.FromRgb(251, 191, 36));
            }
        }
        catch
        {
            LblPing.Text = "-- ms";
        }
    }

    // --- APK DOWNLOADER / INSTALLER ACTIONS (Kullanıcı Talebi) ---
    private async void BtnSideloadApk_Click(object sender, RoutedEventArgs e)
    {
        string? apkPath = _apkManager.LocateApkFile();
        if (string.IsNullOrEmpty(apkPath))
        {
            LblApkStatus.Text = "⚠️ MobilModem.apk henüz derlenmedi veya klasörde bulunamadı.";
            LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(251, 191, 36));
            return;
        }

        BtnSideloadApk.IsEnabled = false;
        LblApkStatus.Text = "⏳ ADB ile telefona yükleniyor...";
        LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(56, 189, 248));

        await Task.Run(() =>
        {
            bool success = _adbManager.InstallApkToDevice(apkPath, out string outMsg, out string? err);
            Dispatcher.Invoke(() =>
            {
                BtnSideloadApk.IsEnabled = true;
                if (success)
                {
                    LblApkStatus.Text = "✅ MobilModem.apk telefonunuza başarıyla yüklendi!";
                    LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
                    Log("APK başarıyla cihaza kuruldu.");
                }
                else
                {
                    LblApkStatus.Text = $"❌ Yükleme başarısız: {err}";
                    LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(244, 63, 94));
                    Log($"APK yükleme hatası: {err}");
                }
            });
        });
    }

    private void BtnDownloadLink_Click(object sender, RoutedEventArgs e)
    {
        string? apkPath = _apkManager.LocateApkFile();
        if (string.IsNullOrEmpty(apkPath))
        {
            LblApkStatus.Text = "⚠️ MobilModem.apk dosyası henüz derlenmedi veya mevcut değil.";
            LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(251, 191, 36));
            return;
        }

        if (_apkManager.IsServerRunning)
        {
            _apkManager.StopLocalDownloadServer();
            BtnDownloadLink.Content = "🌐 İndirme Sunucusu Başlat";
            LblApkStatus.Text = "İndirme sunucusu durduruldu.";
            LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(161, 161, 170));
            return;
        }

        if (_apkManager.StartLocalDownloadServer(apkPath, out string? url, out string? err))
        {
            BtnDownloadLink.Content = "🛑 Sunucuyu Durdur";
            LblApkStatus.Text = $"🌐 Telefon tarayıcısından indirin:\n{url}";
            LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(52, 211, 153));
            Log($"APK indirme sunucusu başlatıldı: {url}");

            // Open in PC browser optionally
            try
            {
                Process.Start(new ProcessStartInfo(url!) { UseShellExecute = true });
            }
            catch { }
        }
        else
        {
            LblApkStatus.Text = $"❌ Sunucu başlatılamadı: {err}";
            LblApkStatus.Foreground = new SolidColorBrush(Color.FromRgb(244, 63, 94));
        }
    }

    private void BtnToggleLog_Click(object sender, RoutedEventArgs e)
    {
        _isLogExpanded = !_isLogExpanded;
        LogBorder.Visibility = _isLogExpanded ? Visibility.Visible : Visibility.Collapsed;
        BtnToggleLog.Content = _isLogExpanded ? "▲ Olay Günlüğünü Gizle" : "▼ Olay Günlüğünü Göster";
    }

    private void Window_Closing(object sender, System.ComponentModel.CancelEventArgs e)
    {
        _adbPollTimer?.Stop();
        _statsTimer?.Stop();
        _apkManager.StopLocalDownloadServer();
        Rollback();
    }
}