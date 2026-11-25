package com.mlzvpn.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import java.io.File
import java.lang.reflect.Proxy

/**
 * Foreground VpnService hosting the gomobile Go orchestrator (mlz-core.aar).
 * Passes the TUN file descriptor and VpnService.protect(fd) callback to Go
 * so sing-box, Xray-core, and All-In sidecars operate without routing loops.
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
    }

    private var tunInterface: ParcelFileDescriptor? = null

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
        return START_STICKY
    }

    private fun startTunnel(engineMode: String, countryCode: String, batterySaver: Boolean) {
        ensureNotificationChannel()
        startForeground(NOTIF_ID, buildNotification("MLZ VPN متصل و امن است"))

        if (tunInterface == null) {
            val builder = Builder()
                .setSession("MLZ VPN")
                .setMtu(1360)
                .addAddress("172.19.0.1", 30)
                .addRoute("0.0.0.0", 0)
                .addDnsServer("172.19.0.2")

            try {
                builder.addDisallowedApplication(packageName)
            } catch (_: Exception) {
            }

            tunInterface = builder.establish()
        }

        val fd = tunInterface?.fd ?: -1
        if (fd >= 0) {
            invokeGoMobileStart(fd, engineMode, countryCode, batterySaver)
            isRunning = true
        }
    }

    private fun stopTunnel() {
        isRunning = false
        try {
            invokeGoMobileDisconnect()
            tunInterface?.close()
        } catch (_: Exception) {
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

    private fun invokeGoMobileStart(tunFd: Int, engineMode: String, countryCode: String, batterySaver: Boolean) {
        try {
            val mobileClass = Class.forName("mobile.Mobile")
            val protectorClass = Class.forName("mobile.SocketProtector")
            val protector = Proxy.newProxyInstance(
                protectorClass.classLoader,
                arrayOf(protectorClass)
            ) { _, method, args ->
                if (method.name == "protect" && args != null && args.isNotEmpty()) {
                    protect((args[0] as Number).toInt())
                } else {
                    false
                }
            }

            val rulesDir = File(filesDir, "rules").apply { mkdirs() }.absolutePath
            val initMethod = mobileClass.getMethod(
                "init",
                String::class.java,
                String::class.java,
                String::class.java,
                Int::class.javaPrimitiveType,
                protectorClass
            )
            initMethod.invoke(
                null,
                cacheDir.absolutePath,
                rulesDir,
                applicationInfo.nativeLibraryDir,
                tunFd,
                protector
            )

            val connectMethod = mobileClass.getMethod(
                "connect",
                String::class.java,
                String::class.java,
                Boolean::class.javaPrimitiveType
            )
            connectMethod.invoke(null, engineMode, countryCode, batterySaver)
        } catch (_: ClassNotFoundException) {
            // mlz-core.aar has not been bundled into app/android/app/libs yet.
        } catch (_: Exception) {
        }
    }

    private fun invokeGoMobileDisconnect() {
        try {
            val mobileClass = Class.forName("mobile.Mobile")
            mobileClass.getMethod("disconnect").invoke(null)
        } catch (_: Exception) {
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
}
