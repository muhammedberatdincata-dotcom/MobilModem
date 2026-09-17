package com.mobilmodem.app.network

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket

class NetworkBinder(context: Context) {
    private val connectivityManager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    @Volatile private var cellularNetwork: Network? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    private val _isCellularAvailable = MutableStateFlow(false)
    val isCellularAvailable: StateFlow<Boolean> = _isCellularAvailable.asStateFlow()

    fun acquireCellularNetwork() {
        val request = NetworkRequest.Builder()
            .addTransportType(NetworkCapabilities.TRANSPORT_CELLULAR)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build()

        networkCallback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                cellularNetwork = network
                _isCellularAvailable.value = true
            }

            override fun onLost(network: Network) {
                if (cellularNetwork == network) {
                    cellularNetwork = null
                    _isCellularAvailable.value = false
                }
            }
        }
        connectivityManager.requestNetwork(request, networkCallback!!)
    }

    fun releaseCellularNetwork() {
        networkCallback?.let {
            try { connectivityManager.unregisterNetworkCallback(it) } catch (e: Exception) {}
            networkCallback = null
        }
        cellularNetwork = null
        _isCellularAvailable.value = false
    }

    fun createBoundSocket(host: String, port: Int): Socket {
        val network = cellularNetwork ?: throw IllegalStateException("Cellular network not available")
        val socket = network.socketFactory.createSocket()
        socket.connect(InetSocketAddress(host, port), 10000)
        return socket
    }
    
    fun resolveDns(host: String): Array<InetAddress> {
        val network = cellularNetwork ?: throw IllegalStateException("Cellular network not available")
        return network.getAllByName(host)
    }
}
