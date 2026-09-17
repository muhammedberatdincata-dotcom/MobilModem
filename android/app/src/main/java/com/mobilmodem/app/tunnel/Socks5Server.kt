package com.mobilmodem.app.tunnel

import com.mobilmodem.app.network.NetworkBinder
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.InputStream
import java.io.OutputStream
import java.net.ServerSocket
import java.net.Socket
import java.net.InetAddress
import java.util.concurrent.atomic.AtomicLong

class Socks5Server(
    private val networkBinder: NetworkBinder,
    private val listenAddress: String = "0.0.0.0",
    private val listenPort: Int = 10808
) {
    private var serverSocket: ServerSocket? = null
    private var serverJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    private val _totalBytesUp = AtomicLong(0)
    private val _totalBytesDown = AtomicLong(0)
    
    private val _speedStats = MutableStateFlow(SpeedStats(0, 0, 0, 0))
    val speedStats: StateFlow<SpeedStats> = _speedStats.asStateFlow()

    private var isRunning = false
    private var speedJob: Job? = null

    data class SpeedStats(val upSpeed: Long, val downSpeed: Long, val totalUp: Long, val totalDown: Long)

    fun start() {
        if (isRunning) return
        isRunning = true
        
        serverSocket = try {
            ServerSocket(listenPort, 50, InetAddress.getByName(listenAddress))
        } catch (e: Exception) {
            // Fallback to all interfaces if specific address bind fails (e.g. P2P not ready)
            ServerSocket(listenPort, 50, InetAddress.getByName("0.0.0.0"))
        }
        
        serverJob = scope.launch {
            while (isActive && isRunning) {
                try {
                    val clientSocket = serverSocket?.accept() ?: break
                    launch { handleClient(clientSocket) }
                } catch (e: Exception) {
                    if (isRunning) e.printStackTrace()
                }
            }
        }

        speedJob = scope.launch {
            var lastUp = _totalBytesUp.get()
            var lastDown = _totalBytesDown.get()
            while (isActive && isRunning) {
                delay(1000)
                val currUp = _totalBytesUp.get()
                val currDown = _totalBytesDown.get()
                _speedStats.value = SpeedStats(
                    upSpeed = currUp - lastUp,
                    downSpeed = currDown - lastDown,
                    totalUp = currUp,
                    totalDown = currDown
                )
                lastUp = currUp
                lastDown = currDown
            }
        }
    }

    fun stop() {
        isRunning = false
        serverJob?.cancel()
        speedJob?.cancel()
        serverSocket?.close()
        scope.coroutineContext.cancelChildren()
    }

    private suspend fun handleClient(clientSocket: Socket) = withContext(Dispatchers.IO) {
        var targetSocket: Socket? = null
        try {
            val input = clientSocket.getInputStream()
            val output = clientSocket.getOutputStream()

            // 1. SOCKS5 Handshake
            val version = input.read()
            if (version != 5) return@withContext
            
            val nMethods = input.read()
            val methods = ByteArray(nMethods)
            input.read(methods)

            // 2. Respond NO AUTH (0x00)
            output.write(byteArrayOf(0x05, 0x00))
            output.flush()

            // 3. Request
            val reqVersion = input.read()
            val cmd = input.read()
            val rsv = input.read()
            val atyp = input.read()

            if (reqVersion != 5 || cmd != 1) return@withContext // Only CONNECT supported

            val destHost: String
            when (atyp) {
                1 -> { // IPv4
                    val ip = ByteArray(4)
                    input.read(ip)
                    destHost = InetAddress.getByAddress(ip).hostAddress ?: ""
                }
                3 -> { // Domain
                    val len = input.read()
                    val domain = ByteArray(len)
                    input.read(domain)
                    destHost = String(domain)
                }
                4 -> { // IPv6
                    val ip = ByteArray(16)
                    input.read(ip)
                    destHost = InetAddress.getByAddress(ip).hostAddress ?: ""
                }
                else -> return@withContext
            }

            val portHigh = input.read()
            val portLow = input.read()
            val destPort = ((portHigh and 0xFF) shl 8) or (portLow and 0xFF)

            // 4. Connect to target
            try {
                // If domain name, resolve using the network binder's DNS
                val resolvedIp = if (atyp == 3) {
                    networkBinder.resolveDns(destHost).firstOrNull()?.hostAddress ?: destHost
                } else {
                    destHost
                }
                
                targetSocket = networkBinder.createBoundSocket(resolvedIp, destPort)
                
                // Reply Success
                val reply = byteArrayOf(0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
                output.write(reply)
                output.flush()
            } catch (e: Exception) {
                // Reply Host Unreachable
                output.write(byteArrayOf(0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00))
                output.flush()
                return@withContext
            }

            // 5. Relay
            val targetInput = targetSocket.getInputStream()
            val targetOutput = targetSocket.getOutputStream()

            val upJob = launch { relay(input, targetOutput, _totalBytesUp) }
            val downJob = launch { relay(targetInput, output, _totalBytesDown) }

            joinAll(upJob, downJob)

        } catch (e: Exception) {
            // Ignore relay closure exceptions
        } finally {
            try { clientSocket.close() } catch (e: Exception) {}
            try { targetSocket?.close() } catch (e: Exception) {}
        }
    }

    private fun relay(input: InputStream, output: OutputStream, counter: AtomicLong) {
        val buffer = ByteArray(16 * 1024)
        try {
            var bytesRead: Int
            while (input.read(buffer).also { bytesRead = it } != -1) {
                output.write(buffer, 0, bytesRead)
                output.flush()
                counter.addAndGet(bytesRead.toLong())
            }
        } catch (e: Exception) {
            // End of stream or connection closed
        }
    }
}
