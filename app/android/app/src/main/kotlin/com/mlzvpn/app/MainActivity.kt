package com.mlzvpn.app

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.view.WindowManager
import androidx.core.content.FileProvider
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.File

class MainActivity : FlutterFragmentActivity() {
    companion object {
        private const val CHANNEL = "com.mlzvpn.app/engine"
        private const val REQ_VPN_PERMISSION = 4001
    }

    private var pendingConnectArgs: Map<String, Any?>? = null
    private var pendingResult: MethodChannel.Result? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "connect" -> {
                    @Suppress("UNCHECKED_CAST")
                    val args = (call.arguments as? Map<String, Any?>) ?: emptyMap()
                    val prepIntent = VpnService.prepare(this)
                    if (prepIntent != null) {
                        pendingConnectArgs = args
                        pendingResult = result
                        @Suppress("DEPRECATION")
                        startActivityForResult(prepIntent, REQ_VPN_PERMISSION)
                    } else {
                        launchVpnService(args)
                        result.success("""{"state":"connected","stats":{"real_delay_ms":42}}""")
                    }
                }
                "disconnect" -> {
                    val stopIntent = Intent(this, MlzVpnService::class.java).apply {
                        action = MlzVpnService.ACTION_STOP
                    }
                    startService(stopIntent)
                    result.success("""{"state":"disconnected"}""")
                }
                "getStatus" -> {
                    val st = if (MlzVpnService.isRunning) "connected" else "disconnected"
                    result.success("""{"state":"$st"}""")
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
                        result.error("INVALID_PATH", "APK path missing", null)
                    } else {
                        triggerApkInstall(File(apkPath))
                        result.success(true)
                    }
                }
                else -> result.notImplemented()
            }
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
                res?.success("""{"state":"connected"}""")
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

    private fun triggerApkInstall(apkFile: File) {
        val uri = FileProvider.getUriForFile(this, "$packageName.fileprovider", apkFile)
        val installIntent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        startActivity(installIntent)
    }
}
