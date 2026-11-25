package com.mlzvpn.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import org.json.JSONObject

/**
 * Foreground VpnService hosting the gomobile Go orchestrator (mlz-core.aar).
 * Passes the TUN file descriptor and VpnService.protect(fd) callback to Go so
 * sing-box/Xray can operate without routing loops.
 *
 * If the native core is absent or refuses to connect, the service closes the
 * TUN descriptor and reports a Persian error. It never marks itself connected
 * while traffic would be blackholed.
 */
class MlzVpnService : VpnService() {

    companion object {
        const val ACTION_START = "com.mlzvpn.app.START"
        const val ACTION_STOP = "com.mlzvpn.app.STOP"
        const val CHANNEL_ID = "mlz_vpn_tunnel"
        const val NOTIF_ID = 1001

        @Volatile
        var isRunning: Boolean = false
            private set

        @Volatile
        var lastError: String? = null
            private set

        @Volatile
        var isStarting: Boolean = false
            private set
    }

    private var tunInterface: ParcelFileDescriptor? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopTunnel()
                return START_NOT_STICKY
            }
            ACTION_START -> {
                val engineMode = intent.getStringExtra("engine_mode") ?: "auto"
                val countryCode = intent.getStringExtra("country_code") ?: "auto"
                val batterySaver = intent.getBooleanExtra("battery_saver", true)
                startTunnel(engineMode, countryCode, batterySaver)
            }
        }
        return START_NOT_STICKY
    }

    private fun startTunnel(engineMode: String, countryCode: String, batterySaver: Boolean) {
        if (isStarting || isRunning) return
        isStarting = true
        ensureNotificationChannel()
        startForeground(NOTIF_ID, buildNotification("در حال آماده‌سازی هسته..."))
        if (!MlzCoreBridge.available()) {
            failClosed("هسته بومی اندروید (mlz-core.aar) در این بسته وجود ندارد؛ اتصال به‌صورت امن مسدود شد.")
            return
        }

        Thread {
            try {
                if (tunInterface == null) {
                    tunInterface = Builder()
                        .setSession("MLZ VPN")
                        .setMtu(1360)
                        .addAddress("172.19.0.1", 30)
                        .addAddress("fdfe:dcba:9876::1", 126)
                        .addRoute("0.0.0.0", 0)
                        // IPv6 is routed into the tunnel so it cannot leak around
                        // the IPv4-only dataplane; apps fall back to IPv4.
                        .addRoute("::", 0)
                        .addDnsServer("172.19.0.2")
                        .apply {
                            try {
                                addDisallowedApplication(packageName)
                            } catch (_: Exception) {
                            }
                        }
                        .establish()
                }

                val fd = tunInterface?.fd ?: -1
                if (fd < 0) {
                    throw IllegalStateException("ساخت رابط TUN توسط سیستم‌عامل رد شد.")
                }

                MlzCoreBridge.init(this, fd)
                val rawStatus = MlzCoreBridge.connect(engineMode, countryCode, batterySaver)
                val status = JSONObject(rawStatus)
                val error = status.optString("error").ifBlank {
                    status.optString("error_message_fa")
                }
                if (error.isNotBlank()) {
                    throw IllegalStateException(error)
                }
                if (status.optString("state") != "connected") {
                    throw IllegalStateException("هسته به وضعیت متصل نرسید.")
                }

                isRunning = true
                lastError = null
                updateNotification("MLZ VPN متصل و امن است")
                startNetworkMonitor()
            } catch (e: Throwable) {
                failClosed(e.message ?: "راه‌اندازی هسته اندروید ناموفق بود.")
            } finally {
                isStarting = false
            }
        }.start()
    }

    private fun failClosed(message: String) {
        stopNetworkMonitor()
        isRunning = false
        isStarting = false
        lastError = message
        try {
            MlzCoreBridge.disconnect()
        } catch (_: Throwable) {
        }
        try {
            tunInterface?.close()
        } catch (_: Throwable) {
        } finally {
            tunInterface = null
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
        stopSelf()
    }

    private fun stopTunnel() {
        stopNetworkMonitor()
        isRunning = false
        isStarting = false
        try {
            MlzCoreBridge.disconnect()
        } catch (_: Throwable) {
        }
        try {
            tunInterface?.close()
        } catch (_: Throwable) {
        } finally {
            tunInterface = null
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
        stopSelf()
    }

    private fun startNetworkMonitor() {
        if (networkCallback != null) return
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                MlzCoreBridge.updateNetwork(this@MlzVpnService)
            }

            override fun onLost(network: Network) {
                MlzCoreBridge.updateNetwork(this@MlzVpnService)
            }

            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                MlzCoreBridge.updateNetwork(this@MlzVpnService)
            }
        }
        try {
            val request = NetworkRequest.Builder()
                .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                .build()
            getSystemService(ConnectivityManager::class.java)
                .registerNetworkCallback(request, callback)
            networkCallback = callback
        } catch (_: Throwable) {
            networkCallback = null
        }
    }

    private fun stopNetworkMonitor() {
        val callback = networkCallback ?: return
        networkCallback = null
        try {
            getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(callback)
        } catch (_: Throwable) {
        }
    }

    private fun ensureNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val mgr = getSystemService(NotificationManager::class.java)
            val ch = NotificationChannel(
                CHANNEL_ID,
                "MLZ VPN Tunnel",
                NotificationManager.IMPORTANCE_LOW
            )
            mgr.createNotificationChannel(ch)
        }
    }

    private fun updateNotification(text: String) {
        val mgr = getSystemService(NotificationManager::class.java)
        mgr.notify(NOTIF_ID, buildNotification(text))
    }

    private fun buildNotification(text: String): Notification {
        val stopIntent = Intent(this, MlzVpnService::class.java).apply { action = ACTION_STOP }
        val stopPending = PendingIntent.getService(
            this, 0, stopIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("MLZ VPN")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setOngoing(true)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "قطع اتصال", stopPending)
            .build()
    }

    override fun onRevoke() {
        stopTunnel()
        super.onRevoke()
    }

    override fun onDestroy() {
        stopNetworkMonitor()
        // Even a service destroyed mid-start must tear down the Go dataplane, or
        // the next Connect could reuse a half-initialized orchestrator.
        if (isRunning || isStarting) {
            stopTunnel()
        }
        super.onDestroy()
    }
}
