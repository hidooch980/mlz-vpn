package com.mlzvpn.app

import android.Manifest
import android.app.Activity
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.provider.Settings
import android.view.WindowManager
import android.widget.Toast
import androidx.core.app.NotificationCompat
import androidx.core.content.FileProvider
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.json.JSONObject
import java.io.File

class MainActivity : FlutterFragmentActivity() {
    companion object {
        private const val CHANNEL = "com.mlzvpn.app/engine"
        private const val REQ_VPN_PERMISSION = 4001
        private const val REQ_NOTIFICATION_PERMISSION = 4002
        private const val UPDATE_CHANNEL = "mlz_app_updates"
        private const val UPDATE_NOTIFICATION_ID = 4100
    }

    private var pendingConnectArgs: Map<String, Any?>? = null
    private var pendingResult: MethodChannel.Result? = null
    private var pendingInstallPath: String? = null
    private var awaitingInstallPermission = false

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        requestNotificationPermission()

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "connect" -> handleConnect(call.arguments as? Map<String, Any?> ?: emptyMap(), result)
                "disconnect" -> {
                    val stopIntent = Intent(this, MlzVpnService::class.java).apply {
                        action = MlzVpnService.ACTION_STOP
                    }
                    startService(stopIntent)
                    result.success("""{"state":"disconnected"}""")
                }
                "getStatus" -> result.success(currentStatus())
                "testIpHealth" -> jsonCall(result) { requireCore(); MlzCoreBridge.testIpHealth() }
                // Diagnostics can run without a live VPN: the Go bridge falls
                // back to local network/DNS probes when the tunnel is stopped.
                "runDiagnostics" -> jsonCall(result) { MlzCoreBridge.runDiagnostics() }
                "autoHealDiagnostics" -> jsonCall(result) { requireCore(); MlzCoreBridge.autoHealDiagnostics() }
                "toggleLan" -> {
                    val enabled = call.argument<Boolean>("enabled") ?: false
                    val usePin = call.argument<Boolean>("use_pin") ?: false
                    jsonCall(result) { requireCore(); MlzCoreBridge.toggleLan(enabled, usePin) }
                }
                "checkUpdate" -> jsonCall(result) {
                    if (MlzCoreBridge.available() && MlzCoreBridge.initialized) {
                        MlzCoreBridge.checkUpdate()
                    } else {
                        MlzCoreBridge.checkUpdateStandalone(this)
                    }
                }
                "setSecureFlag" -> {
                    val secure = (call.argument<Boolean>("secure")) ?: false
                    if (secure) {
                        window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
                    } else {
                        window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
                    }
                    result.success(true)
                }
                "installApk" -> {
                    val apkPath = call.argument<String>("path")
                    if (apkPath.isNullOrBlank()) {
                        result.error("INVALID_PATH", "مسیر فایل به‌روزرسانی خالی است.", null)
                    } else {
                        result.success(triggerApkInstall(File(apkPath)))
                    }
                }
                "downloadUpdate" -> handleDownloadUpdate(call, result)
                else -> result.notImplemented()
            }
        }
    }

    private fun requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), REQ_NOTIFICATION_PERMISSION)
        }
    }

    private fun handleConnect(args: Map<String, Any?>, result: MethodChannel.Result) {
        if (!MlzCoreBridge.available()) {
            result.error(
                "CORE_UNAVAILABLE",
                "هسته بومی اندروید (mlz-core.aar) در این بسته وجود ندارد؛ اتصال به‌صورت امن مسدود شد.",
                null
            )
            return
        }
        val prepIntent = VpnService.prepare(this)
        if (prepIntent != null) {
            pendingConnectArgs = args
            pendingResult = result
            @Suppress("DEPRECATION")
            startActivityForResult(prepIntent, REQ_VPN_PERMISSION)
        } else {
            launchVpnService(args)
            result.success("""{"state":"connecting"}""")
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_VPN_PERMISSION) {
            val res = pendingResult
            val args = pendingConnectArgs ?: emptyMap()
            pendingResult = null
            pendingConnectArgs = null
            if (resultCode == Activity.RESULT_OK) {
                launchVpnService(args)
                res?.success("""{"state":"connecting"}""")
            } else {
                res?.error("VPN_DENIED", "مجوز ایجاد اتصال VPN توسط کاربر تایید نشد", null)
            }
        }
    }

    private fun launchVpnService(args: Map<String, Any?>) {
        val intent = Intent(this, MlzVpnService::class.java).apply {
            action = MlzVpnService.ACTION_START
            putExtra("engine_mode", (args["engine_mode"] as? String) ?: "auto")
            putExtra("country_code", (args["country_code"] as? String) ?: "auto")
            putExtra("battery_saver", (args["battery_saver"] as? Boolean) ?: true)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun requireCore() {
        if (!MlzCoreBridge.available()) {
            throw IllegalStateException("هسته بومی اندروید در این بسته موجود نیست.")
        }
        if (!MlzCoreBridge.initialized) {
            throw IllegalStateException("هسته اندروید هنوز راه‌اندازی نشده است؛ ابتدا اتصال را برقرار کنید.")
        }
    }

    private fun currentStatus(): String {
        if (MlzVpnService.isRunning && MlzCoreBridge.initialized) {
            try {
                return MlzCoreBridge.status()
            } catch (e: Throwable) {
                val obj = JSONObject()
                obj.put("state", "disconnected")
                obj.put("error_message_fa", e.message ?: "وضعیت هسته اندروید خوانده نشد.")
                return obj.toString()
            }
        }
        val obj = JSONObject()
        obj.put("state", if (MlzVpnService.isStarting) "connecting" else "disconnected")
        MlzVpnService.lastError?.takeIf { it.isNotBlank() }?.let {
            obj.put("error_message_fa", it)
        }
        return obj.toString()
    }

    private inline fun jsonCall(result: MethodChannel.Result, block: () -> String) {
        try {
            result.success(block())
        } catch (e: Throwable) {
            result.error("ENGINE_ERROR", e.message ?: "خطای هسته اندروید", null)
        }
    }

    private fun handleDownloadUpdate(call: MethodCall, result: MethodChannel.Result) {
        val url = call.argument<String>("url")?.trim().orEmpty()
        val sha256 = call.argument<String>("sha256")?.trim().orEmpty()
        if (url.isEmpty() || sha256.isEmpty()) {
            result.error("INVALID_UPDATE", "اطلاعات به‌روزرسانی ناقص است.", null)
            return
        }
        ensureUpdateChannel()
        showUpdateNotification("در حال دانلود نسخه جدید MLZ VPN...", true, null)
        Thread {
            try {
                val raw = MlzCoreBridge.downloadUpdate(cacheDir.absolutePath, url, sha256)
                val obj = JSONObject(raw)
                if (obj.has("error")) {
                    throw IllegalStateException(obj.optString("error"))
                }
                val path = obj.optString("path")
                val apk = File(path)
                if (!apk.isFile || apk.length() <= 0L) {
                    throw IllegalStateException("فایل به‌روزرسانی پس از دانلود پیدا نشد.")
                }
                runOnUiThread {
                    showUpdateNotification("نسخه جدید دانلود شد؛ برای نصب ضربه بزنید.", false, apk)
                    val started = triggerApkInstall(apk)
                    val payload = JSONObject()
                        .put("path", path)
                        .put("bytes", apk.length())
                        .put("install_started", started)
                    result.success(payload.toString())
                }
            } catch (e: Throwable) {
                runOnUiThread {
                    showUpdateNotification("دریافت به‌روزرسانی ناموفق بود.", false, null)
                    result.error("UPDATE_FAILED", e.message ?: "دریافت به‌روزرسانی ناموفق بود.", null)
                }
            }
        }.start()
    }

    private fun ensureUpdateChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val mgr = getSystemService(NotificationManager::class.java)
            val channel = NotificationChannel(
                UPDATE_CHANNEL,
                "به‌روزرسانی برنامه",
                NotificationManager.IMPORTANCE_DEFAULT
            ).apply {
                description = "اطلاع‌رسانی دانلود و نصب نسخه جدید MLZ VPN"
            }
            mgr.createNotificationChannel(channel)
        }
    }

    private fun showUpdateNotification(text: String, ongoing: Boolean, apk: File?) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
        val openIntent = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        val contentIntent = if (apk != null) {
            installPendingIntent(apk)
        } else {
            PendingIntent.getActivity(
                this,
                0,
                openIntent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
            )
        }
        val notification = NotificationCompat.Builder(this, UPDATE_CHANNEL)
            .setSmallIcon(
                if (ongoing) android.R.drawable.stat_sys_download
                else android.R.drawable.stat_sys_download_done
            )
            .setContentTitle("MLZ VPN")
            .setContentText(text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(text))
            .setContentIntent(contentIntent)
            .setOngoing(ongoing)
            .setAutoCancel(!ongoing)
            .setOnlyAlertOnce(ongoing)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .build()
        getSystemService(NotificationManager::class.java).notify(UPDATE_NOTIFICATION_ID, notification)
    }

    private fun installPendingIntent(apk: File): PendingIntent {
        val uri = FileProvider.getUriForFile(this, "$packageName.fileprovider", apk)
        val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        return PendingIntent.getActivity(
            this,
            42,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
    }

    private fun canInstallPackages(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.O || packageManager.canRequestPackageInstalls()

    private fun triggerApkInstall(apkFile: File): Boolean {
        if (!apkFile.isFile || apkFile.length() <= 0L) {
            Toast.makeText(this, "فایل به‌روزرسانی معتبر نیست.", Toast.LENGTH_LONG).show()
            return false
        }
        if (!canInstallPackages()) {
            pendingInstallPath = apkFile.absolutePath
            awaitingInstallPermission = true
            Toast.makeText(
                this,
                "برای نصب به‌روزرسانی، اجازه نصب برنامه‌ها را برای MLZ VPN فعال کنید.",
                Toast.LENGTH_LONG
            ).show()
            startActivity(
                Intent(
                    Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:$packageName")
                )
            )
            return false
        }
        return try {
            val uri = FileProvider.getUriForFile(this, "$packageName.fileprovider", apkFile)
            val installIntent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            startActivity(installIntent)
            true
        } catch (e: Throwable) {
            Toast.makeText(this, "بازکردن نصب‌کننده ناموفق بود: " + (e.message ?: "خطای نامشخص"), Toast.LENGTH_LONG).show()
            false
        }
    }

    override fun onResume() {
        super.onResume()
        if (!awaitingInstallPermission) return
        awaitingInstallPermission = false
        val path = pendingInstallPath
        pendingInstallPath = null
        if (path.isNullOrBlank()) return
        if (canInstallPackages()) {
            triggerApkInstall(File(path))
        } else {
            Toast.makeText(
                this,
                "اجازه نصب برنامه‌ها فعال نشد؛ از تنظیمات، نصب از این برنامه را مجاز کنید.",
                Toast.LENGTH_LONG
            ).show()
        }
    }
}
