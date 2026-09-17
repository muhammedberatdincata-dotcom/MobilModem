package com.mobilmodem.app.p2p

import android.annotation.SuppressLint
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.wifi.p2p.WifiP2pConfig
import android.net.wifi.p2p.WifiP2pGroup
import android.net.wifi.p2p.WifiP2pManager
import android.os.Build
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.UUID

data class P2pGroupInfo(
    val ssid: String,
    val passphrase: String,
    val goAddress: String,
    val clientCount: Int
)

@SuppressLint("MissingPermission")
class P2pServerManager(private val context: Context) {
    private val wifiP2pManager: WifiP2pManager = context.getSystemService(Context.WIFI_P2P_SERVICE) as WifiP2pManager
    private var channel: WifiP2pManager.Channel? = null
    private var receiver: BroadcastReceiver? = null

    private val _groupInfo = MutableStateFlow<P2pGroupInfo?>(null)
    val groupInfo: StateFlow<P2pGroupInfo?> = _groupInfo.asStateFlow()

    fun initialize() {
        channel = wifiP2pManager.initialize(context, context.mainLooper, null)
    }

    fun startGroup() {
        val ch = channel ?: return
        wifiP2pManager.removeGroup(ch, object : WifiP2pManager.ActionListener {
            override fun onSuccess() {
                createP2pGroup()
            }
            override fun onFailure(reason: Int) {
                createP2pGroup()
            }
        })
        
        receiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                if (intent?.action == WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION) {
                    requestInfo()
                }
            }
        }
        context.registerReceiver(receiver, IntentFilter(WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION))
    }

    private fun createP2pGroup() {
        val ch = channel ?: return
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val config = WifiP2pConfig.Builder()
                .setNetworkName("DIRECT-mm-MobilModem")
                .setPassphrase(UUID.randomUUID().toString().substring(0, 12))
                .enablePersistentMode(false)
                .build()
            wifiP2pManager.createGroup(ch, config, object : WifiP2pManager.ActionListener {
                override fun onSuccess() { requestInfo() }
                override fun onFailure(reason: Int) { }
            })
        } else {
            wifiP2pManager.createGroup(ch, object : WifiP2pManager.ActionListener {
                override fun onSuccess() { requestInfo() }
                override fun onFailure(reason: Int) { }
            })
        }
    }

    private fun requestInfo() {
        val ch = channel ?: return
        wifiP2pManager.requestGroupInfo(ch) { group: WifiP2pGroup? ->
            if (group != null) {
                wifiP2pManager.requestConnectionInfo(ch) { info ->
                    val goAddress = info?.groupOwnerAddress?.hostAddress ?: "192.168.49.1"
                    _groupInfo.value = P2pGroupInfo(
                        ssid = group.networkName,
                        passphrase = group.passphrase ?: "",
                        goAddress = goAddress,
                        clientCount = group.clientList.size
                    )
                }
            } else {
                _groupInfo.value = null
            }
        }
    }

    fun stopGroup() {
        val ch = channel ?: return
        wifiP2pManager.removeGroup(ch, null)
        receiver?.let {
            try { context.unregisterReceiver(it) } catch (e: Exception) {}
            receiver = null
        }
        _groupInfo.value = null
    }
}
