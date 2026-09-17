package com.mobilmodem.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.wifi.WifiManager
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import androidx.core.app.NotificationCompat
import com.mobilmodem.app.MainActivity
import com.mobilmodem.app.network.NetworkBinder
import com.mobilmodem.app.p2p.P2pGroupInfo
import com.mobilmodem.app.p2p.P2pServerManager
import com.mobilmodem.app.tunnel.Socks5Server
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class TunnelServerService : Service() {

    companion object {
        const val ACTION_START = "ACTION_START"
        const val ACTION_STOP = "ACTION_STOP"
        const val EXTRA_MODE = "EXTRA_MODE"
        const val MODE_USB = "usb"
        const val MODE_P2P = "p2p"

        private const val NOTIFICATION_ID = 101
        private const val CHANNEL_ID = "mobilmodem_tunnel_channel"

        private val _isRunning = MutableStateFlow(false)
        val isRunning: StateFlow<Boolean> = _isRunning.asStateFlow()

        private val _currentMode = MutableStateFlow<String?>(null)
        val currentMode: StateFlow<String?> = _currentMode.asStateFlow()

        private var activeSocksServer: Socks5Server? = null
        private var activeP2pManager: P2pServerManager? = null

        val speedStats: StateFlow<Socks5Server.SpeedStats>
            get() = activeSocksServer?.speedStats ?: MutableStateFlow(Socks5Server.SpeedStats(0, 0, 0, 0))

        val p2pGroupInfo: StateFlow<P2pGroupInfo?>
            get() = activeP2pManager?.groupInfo ?: MutableStateFlow(null)

        fun start(context: Context, mode: String) {
            val intent = Intent(context, TunnelServerService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_MODE, mode)
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: Context) {
            val intent = Intent(context, TunnelServerService::class.java).apply {
                action = ACTION_STOP
            }
            context.startService(intent)
        }
    }

    private var wakeLock: PowerManager.WakeLock? = null
    private var wifiLock: WifiManager.WifiLock? = null
    private val serviceScope = CoroutineScope(Dispatchers.Default + SupervisorJob())

    private var networkBinder: NetworkBinder? = null
    private var socks5Server: Socks5Server? = null
    private var p2pServerManager: P2pServerManager? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val mode = intent.getStringExtra(EXTRA_MODE) ?: MODE_USB
                startServer(mode)
            }
            ACTION_STOP -> {
                stopServer()
                stopSelf()
            }
        }
        return START_STICKY
    }

    private fun startServer(mode: String) {
        if (_isRunning.value) return

        acquireLocks()

        val notification = buildNotification(
            title = "MobilModem Tüneli Başlatılıyor...",
            content = "Hücresel arayüz hazırlanıyor..."
        )

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE or ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }

        networkBinder = NetworkBinder(this).apply { acquireCellularNetwork() }

        val listenIp = if (mode == MODE_P2P) "192.168.49.1" else "0.0.0.0"
        socks5Server = Socks5Server(networkBinder!!, listenIp, 10808).apply { start() }
        activeSocksServer = socks5Server

        if (mode == MODE_P2P) {
            p2pServerManager = P2pServerManager(this).apply {
                initialize()
                startGroup()
            }
            activeP2pManager = p2pServerManager
        }

        _isRunning.value = true
        _currentMode.value = mode

        // Realtime notification update loop
        serviceScope.launch {
            socks5Server?.speedStats?.collect { stats ->
                val upMb = String.format("%.2f", stats.upSpeed / 1024f / 1024f)
                val dnMb = String.format("%.2f", stats.downSpeed / 1024f / 1024f)
                val totalMb = String.format("%.1f", (stats.totalUp + stats.totalDown) / 1024f / 1024f)

                val modeDesc = if (mode == MODE_P2P) "Wi-Fi Direct" else "USB Ultra Hız"
                updateNotification(
                    title = "MobilModem Aktif ($modeDesc)",
                    content = "↓ $dnMb MB/s  ↑ $upMb MB/s  | Toplam: $totalMb MB"
                )
            }
        }
    }

    private fun stopServer() {
        _isRunning.value = false
        _currentMode.value = null

        serviceScope.coroutineContext.cancelChildren()

        socks5Server?.stop()
        activeSocksServer = null

        p2pServerManager?.stopGroup()
        activeP2pManager = null

        networkBinder?.releaseCellularNetwork()
        networkBinder = null

        releaseLocks()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            stopForeground(true)
        }
    }

    private fun acquireLocks() {
        val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "MobilModem::WakeLock").apply {
            setReferenceCounted(false)
            acquire(24 * 60 * 60 * 1000L) // 24h
        }

        val wm = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        wifiLock = wm.createWifiLock(
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                WifiManager.WIFI_MODE_FULL_HIGH_PERF
            } else {
                @Suppress("DEPRECATION")
                WifiManager.WIFI_MODE_FULL
            },
            "MobilModem::WifiLock"
        ).apply {
            setReferenceCounted(false)
            acquire()
        }
    }

    private fun releaseLocks() {
        wakeLock?.let { if (it.isHeld) it.release() }
        wifiLock?.let { if (it.isHeld) it.release() }
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "MobilModem Tünel Servisi",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Hotspot bypass tünel trafiğini ve anlık aktarım hızını gösterir"
                setShowBadge(false)
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(title: String, content: String): Notification {
        val openIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val stopIntent = PendingIntent.getService(
            this,
            1,
            Intent(this, TunnelServerService::class.java).apply { action = ACTION_STOP },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(content)
            .setSmallIcon(android.R.drawable.stat_sys_upload_done)
            .setContentIntent(openIntent)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Tüneli Durdur", stopIntent)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    private fun updateNotification(title: String, content: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification(title, content))
    }

    override fun onTimeout(startId: Int) {
        stopServer()
        stopSelf()
    }

    @Suppress("OVERRIDE_DEPRECATION")
    fun onTimeout(startId: Int, fgsType: Int) {
        stopServer()
        stopSelf()
    }

    override fun onDestroy() {
        stopServer()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}
