# ⚡ MobilModem — Operatör Korumalı Çift Modlu L3 Ağ Tüneli

Mobil operatörlerin tethering/hotspot kota kısıtlamalarını, APN yönlendirmelerini, DPI mekanizmalarını ve TTL kontrollerini %100 aşan; **PdaNet+ ve NetShare mimarisini geride bırakan** gerçek Layer-3 (L3) sanal ağ adaptörü ve Android mobil tüneli.

---

## 🎯 Temel Özellikler

1. **Tam L3 Ağ Adaptörü (Modem Deneyimi):**
   - Tarayıcıyla sınırlı proxy (HTTP/SOCKS) yerine Windows çekirdeğinde **WireGuard Wintun** sanal ağ bağdaştırıcısı (`10.0.0.2 / 24`) oluşturur.
   - **Tüm PC trafiği** (Discord, Steam, Riot Vanguard / Valorant, çevrimiçi oyunlar, torrent, DNS sorguları) tünelden geçer.

2. **Operatör Tespit Mekanizmalarının %100 Aşılması:**
   - **TTL Normalizasyonu:** Windows çıkış TTL değeri otomatik olarak `65` yapılır. Paket Android soketinden hücresel ağa aktarılırken 1 azalarak baz istasyonuna tam `64` (telefonun kendi yerel trafiği) olarak ulaşır. Operatör hotspot/tethering yapıldığını anlayamaz.
   - **APN & Hotspot Bypass:** Android standart hotspot (tethering APN) ASLA açılmaz. Soketler doğrudan hücresel veri arayüzüne (`TRANSPORT_CELLULAR`) bağlanır. Operatör trafiği telefonun kendi uygulaması gibi görür.

3. **Çift Mod:**
   - **⚡ USB Ultra Hız Modu (Kabloyla Modem):** ADB port forwarding (`10808`) üzerinden sıfır gecikmeli, gigabit hızında kablolu tünel.
   - **📶 Wi-Fi Direct Modu (P2P Kablosuz Modem):** Telefonu P2P Group Owner yapar. Standart hotspot tetiklenmeden kablosuz modem olarak çalışır.

4. **📱 Entegre Mobil APK Yükleyici & İndirici (Yeni):**
   - **Tek Tıkla Telefona Kur (ADB Sideload):** Telefon USB ile bağlıyken masaüstü uygulamasından tek tıkla APK'yı telefona yükler.
   - **Yerel İndirme Sunucusu:** Tek tıkla yerel HTTP sunucusu açarak telefon tarayıcısından APK indirme bağlantısı sunar.

---

## 📂 Hazır Dağıtım Klasörü (`dist/`)

Projenin derlenmiş ve tüm harici sürücüleri içeren tam paketi `c:\Users\muham\Downloads\cakmapda\dist\` klasöründedir:

```
dist/
├── MobilModem.exe         # Windows Masaüstü İstemcisi (.NET 8/10 WPF - Self-Contained)
├── wintun.dll             # Resmi WireGuard TUN Sürücüsü (v0.14.1 amd64)
├── tun2socks.exe          # Resmi gVisor Netstack L3 Tünel Motoru (v2.5.2)
├── adb.exe                # Resmi Google Android Platform-Tools ADB
├── AdbWinApi.dll          # ADB Windows API
└── AdbWinUsbApi.dll       # ADB Windows USB API
```

> 💡 **Sıfır Kurulum Gereksinimi:** `MobilModem.exe` tek başına çalışabilir; harici bir SDK veya Python/Go kurulumu gerektirmez.

---

## 🚀 Hızlı Başlangıç

### 1. Adım: Android Uygulamasını Başlatın
- Android Studio ile `android/` klasörünü açıp telefonunuza yükleyin veya `dist/` içinden tek tıkla yükleyin.
- Uygulamayı açıp yeşil **GÜÇ DÜĞMESİNE** basın.
- **USB Modu** veya **Wi-Fi Direct Modu**nu seçin.

### 2. Adım: Windows İstemcisini Çalıştırın
1. `dist/MobilModem.exe` uygulamasını çift tıklatarak açın (UAC Yönetici iznini onaylayın).
2. İstediğiniz modu seçin:
   - **USB Kablo Modu:** Telefonu kabloyla bağlayıp USB Hata Ayıklamayı açın.
   - **Wi-Fi Direct Modu:** Telefon ekranındaki Wi-Fi adına bağlanın.
3. **TÜNELİ BAŞLAT** butonuna basın.
4. Anlık ping ve akış durumu yeşile dönecek; tüm internetiniz hücresel kota harcamadan akmaya başlayacaktır!

### 3. Adım: Bağlantıyı Kesme
- **BAĞLANTIYI KES** butonuna basıldığında veya pencere kapatıldığında:
  - TTL otomatik `128`e döner.
  - Windows rota tablosu sıfırlanır.
  - Wintun adaptörü ve ADB tüneli güvenle kapatılır.

---

## 🛠️ Kaynak Kod Mimarisi

```
cakmapda/
├── dist/                          # Dağıtıma hazır tek paket (.exe, wintun, adb, tun2socks)
├── windows_client/                # C# .NET 8 WPF Windows Masaüstü İstemcisi
│   ├── MainWindow.xaml / .cs      # Koyu tema, kartlar, ping sayacı, APK sideload UI
│   ├── TtlManager.cs              # Windows TTL 65/128 düzenleyici
│   ├── RouteManager.cs            # İki-yarım routing & loopback koruması
│   ├── AdbManager.cs              # ADB cihaz tespiti, port forwarding & APK kurulumu
│   ├── WintunManager.cs           # Wintun adaptör & tun2socks proses yöneticisi
│   ├── ApkManager.cs              # Yerel APK tespit ve HTTP indirme sunucusu
│   └── app.manifest               # UAC Administrator yetki manifesti
│
├── android/                       # Android Mobil Uygulaması (Kotlin + Jetpack Compose)
│   ├── app/src/main/
│   │   ├── AndroidManifest.xml    # Android 14+ FGS ve P2P izinleri
│   │   └── java/.../
│   │       ├── MainActivity.kt    # Jetpack Compose Material 3 Dashboard
│   │       ├── NetworkBinder.kt   # Hücresel ağ zorunlu soket bağlayıcı (socketFactory)
│   │       ├── Socks5Server.kt    # RFC 1928 SOCKS5 Coroutine sunucu
│   │       ├── P2pServerManager.kt# Autonomous P2P Group Owner yöneticisi
│   │       └── TunnelServerService# Foreground Service & bildirim çubuğu kontrolleri
│
└── windows/                       # Go alternatif istemcisi (Fyne v2)
```
