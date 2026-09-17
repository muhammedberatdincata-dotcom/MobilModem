package com.mobilmodem.app

import android.Manifest
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.*
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.mobilmodem.app.service.TunnelServerService
import com.mobilmodem.app.tunnel.Socks5Server

class MainActivity : ComponentActivity() {

    private val requestPermissionsLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { _ -> }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        requestNecessaryPermissions()

        setContent {
            MaterialTheme(colorScheme = darkColorScheme(
                primary = Color(0xFF10B981),
                onPrimary = Color.Black,
                primaryContainer = Color(0xFF064E3B),
                onPrimaryContainer = Color(0xFF6EE7B7),
                surface = Color(0xFF18181B),
                background = Color(0xFF09090B),
                error = Color(0xFFEF4444)
            )) {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    MobilModemDashboard()
                }
            }
        }
    }

    private fun requestNecessaryPermissions() {
        val perms = mutableListOf<String>()

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            perms.add(Manifest.permission.NEARBY_WIFI_DEVICES)
            perms.add(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            perms.add(Manifest.permission.ACCESS_FINE_LOCATION)
            perms.add(Manifest.permission.ACCESS_COARSE_LOCATION)
        }

        requestPermissionsLauncher.launch(perms.toTypedArray())

        // Request battery optimization exemption for uninterrupted background sharing
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
            if (!pm.isIgnoringBatteryOptimizations(packageName)) {
                try {
                    val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
                        data = Uri.parse("package:$packageName")
                    }
                    startActivity(intent)
                } catch (_: Exception) {}
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MobilModemDashboard() {
    val context = LocalContext.current
    val isRunning by TunnelServerService.isRunning.collectAsState(initial = false)
    val currentMode by TunnelServerService.currentMode.collectAsState(initial = null)
    var selectedMode by remember { mutableStateOf(TunnelServerService.MODE_USB) }

    val speedStats by TunnelServerService.speedStats.collectAsState(initial = Socks5Server.SpeedStats(0, 0, 0, 0))
    val p2pGroupInfo by TunnelServerService.p2pGroupInfo.collectAsState(initial = null)

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(
                            imageVector = Icons.Default.Bolt,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(28.dp)
                        )
                        Spacer(Modifier.width(8.dp))
                        Text("MobilModem", fontWeight = FontWeight.Bold)
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            // 1. Dashboard Status Card with Master Power Toggle
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(20.dp),
                colors = CardDefaults.cardColors(
                    containerColor = if (isRunning) Color(0xFF064E3B) else MaterialTheme.colorScheme.surface
                )
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(20.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Text(
                        text = if (isRunning) "HÜCRESEL AĞ PAYLAŞILIYOR" else "TÜNEL KAPALI",
                        color = if (isRunning) Color(0xFF34D399) else Color.Gray,
                        fontWeight = FontWeight.Bold,
                        fontSize = 14.sp
                    )

                    Spacer(Modifier.height(16.dp))

                    // Big Power Button
                    IconButton(
                        onClick = {
                            if (isRunning) {
                                TunnelServerService.stop(context)
                            } else {
                                TunnelServerService.start(context, selectedMode)
                            }
                        },
                        modifier = Modifier
                            .size(96.dp)
                            .clip(CircleShape)
                            .background(if (isRunning) Color(0xFF10B981) else Color(0xFF27272A))
                    ) {
                        Icon(
                            imageVector = Icons.Default.PowerSettingsNew,
                            contentDescription = "Power",
                            modifier = Modifier.size(48.dp),
                            tint = if (isRunning) Color.Black else Color.White
                        )
                    }

                    Spacer(Modifier.height(12.dp))

                    Text(
                        text = if (isRunning) "Durdurmak için dokunun" else "Başlatmak için dokunun",
                        fontSize = 12.sp,
                        color = Color.LightGray
                    )
                }
            }

            // 2. Mode Selector (Disabled when running)
            TabRow(
                selectedTabIndex = if (selectedMode == TunnelServerService.MODE_USB) 0 else 1,
                containerColor = MaterialTheme.colorScheme.surface,
                contentColor = MaterialTheme.colorScheme.primary,
                modifier = Modifier.clip(RoundedCornerShape(12.dp))
            ) {
                Tab(
                    selected = selectedMode == TunnelServerService.MODE_USB,
                    onClick = { if (!isRunning) selectedMode = TunnelServerService.MODE_USB },
                    text = {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Usb, contentDescription = null, Modifier.size(18.dp))
                            Spacer(Modifier.width(6.dp))
                            Text("USB Modu")
                        }
                    }
                )
                Tab(
                    selected = selectedMode == TunnelServerService.MODE_P2P,
                    onClick = { if (!isRunning) selectedMode = TunnelServerService.MODE_P2P },
                    text = {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Wifi, contentDescription = null, Modifier.size(18.dp))
                            Spacer(Modifier.width(6.dp))
                            Text("Wi-Fi Direct")
                        }
                    }
                )
            }

            // 3. Mode Instruction / Info Card
            AnimatedVisibility(
                visible = selectedMode == TunnelServerService.MODE_USB,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
                ) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.Info, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                            Spacer(Modifier.width(8.dp))
                            Text("USB Ultra Hız Modu Nasıl Kullanılır?", fontWeight = FontWeight.Bold)
                        }
                        Text(
                            "1. Cihazınızda Geliştirici Seçenekleri > USB Hata Ayıklama'yı açın.\n" +
                            "2. Telefonu USB kablo ile bilgisayara bağlayın.\n" +
                            "3. Yukarıdaki yeşil butona basarak tüneli başlatın.\n" +
                            "4. Bilgisayarınızdaki MobilModem.exe uygulamasından 'Bağlan' butonuna basın.",
                            fontSize = 13.sp,
                            color = Color.LightGray
                        )
                    }
                }
            }

            AnimatedVisibility(
                visible = selectedMode == TunnelServerService.MODE_P2P,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
                ) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Icons.Default.WifiTethering, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                            Spacer(Modifier.width(8.dp))
                            Text("Wi-Fi Direct (P2P) Bilgileri", fontWeight = FontWeight.Bold)
                        }

                        if (isRunning && p2pGroupInfo != null) {
                            val info = p2pGroupInfo!!

                            OutlinedCard(Modifier.fillMaxWidth()) {
                                Column(Modifier.padding(12.dp)) {
                                    Text("Ağ Adı (SSID):", fontSize = 12.sp, color = Color.Gray)
                                    Text(info.ssid, fontWeight = FontWeight.Bold, fontSize = 16.sp)

                                    Spacer(Modifier.height(8.dp))

                                    Text("WPA2 Parolası:", fontSize = 12.sp, color = Color.Gray)
                                    Text(info.passphrase, fontWeight = FontWeight.Bold, fontSize = 16.sp, color = MaterialTheme.colorScheme.primary)

                                    Spacer(Modifier.height(10.dp))

                                    Button(
                                        onClick = {
                                            val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                                            cm.setPrimaryClip(ClipData.newPlainText("wifi_password", info.passphrase))
                                            Toast.makeText(context, "Parola kopyalandı!", Toast.LENGTH_SHORT).show()
                                        },
                                        modifier = Modifier.fillMaxWidth(),
                                        colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.primaryContainer)
                                    ) {
                                        Icon(Icons.Default.ContentCopy, contentDescription = null, Modifier.size(16.dp))
                                        Spacer(Modifier.width(8.dp))
                                        Text("Parolayı Kopyala")
                                    }
                                }
                            }

                            Text("Ağ Geçidi IP: ${info.goAddress}:10808", fontSize = 13.sp, color = Color.LightGray)
                            Text("Bağlı İstemci Sayısı: ${info.clientCount}", fontSize = 13.sp, color = Color.LightGray)
                        } else {
                            Text(
                                "Tüneli başlattığınızda telefon bağımsız bir P2P Group Owner ağı oluşturacak. Standart hotspot tetiklenmediği için operatör kotanız etkilenmez.",
                                fontSize = 13.sp,
                                color = Color.LightGray
                            )
                        }
                    }
                }
            }

            // 4. Real-time Traffic Monitoring Card
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
            ) {
                Column(Modifier.padding(16.dp)) {
                    Text("Canlı Ağ Trafiği ve Tüketim", fontWeight = FontWeight.Bold)

                    Spacer(Modifier.height(12.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        // Download Card
                        Column {
                            Text("İndirme (Download)", fontSize = 12.sp, color = Color.Gray)
                            val dnSpeed = String.format("%.2f", speedStats.downSpeed / 1024f / 1024f)
                            Text("↓ $dnSpeed MB/s", fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color(0xFF60A5FA))
                            val dnTotal = String.format("%.1f", speedStats.totalDown / 1024f / 1024f)
                            Text("Toplam: $dnTotal MB", fontSize = 11.sp, color = Color.LightGray)
                        }

                        // Upload Card
                        Column {
                            Text("Yükleme (Upload)", fontSize = 12.sp, color = Color.Gray)
                            val upSpeed = String.format("%.2f", speedStats.upSpeed / 1024f / 1024f)
                            Text("↑ $upSpeed MB/s", fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color(0xFF34D399))
                            val upTotal = String.format("%.1f", speedStats.totalUp / 1024f / 1024f)
                            Text("Toplam: $upTotal MB", fontSize = 11.sp, color = Color.LightGray)
                        }
                    }
                }
            }
        }
    }
}
