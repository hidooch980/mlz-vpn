package com.mlzvpn.app

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.lang.reflect.InvocationTargetException
import java.lang.reflect.Proxy
import java.net.NetworkInterface
import java.util.Collections

/**
 * Reflection facade over gomobile's generated mobile.Mobile API. The AAR is
 * optional at compile time so the Flutter app still builds without the native
 * core; every call fails closed with a Persian error instead of faking a tunnel.
 */
object MlzCoreBridge {
    @Volatile
    var lastError: String? = null
        private set

    @Volatile
    var initialized: Boolean = false
        private set

    fun available(): Boolean = try {
        Class.forName("mobile.Mobile")
        true
    } catch (_: Throwable) {
        false
    }

    private fun fail(message: String): Nothing {
        lastError = message
        throw IllegalStateException(message)
    }

    private fun mobileClass(): Class<*> = try {
        Class.forName("mobile.Mobile")
    } catch (_: Throwable) {
        fail("هسته بومی اندروید (mlz-core.aar) در این بسته وجود ندارد؛ اتصال به‌صورت امن مسدود شد.")
    }

    fun init(context: Context, tunFd: Int): String {
        val cls = mobileClass()
        val protectorClass = Class.forName("mobile.SocketProtector")
        val protector = Proxy.newProxyInstance(
            protectorClass.classLoader,
            arrayOf(protectorClass)
        ) { _, method, args ->
            if (method.name == "protect" && args != null && args.isNotEmpty()) {
                val service = context as? android.net.VpnService
                service?.protect((args[0] as Number).toInt()) ?: false
            } else {
                false
            }
        }
        val rulesDir = File(context.filesDir, "rules").apply { mkdirs() }.absolutePath
        return try {
            val method = cls.getMethod(
                "init",
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                Int::class.javaPrimitiveType,
                protectorClass
            )
            val result = method.invoke(
                null,
                context.cacheDir.absolutePath,
                rulesDir,
                context.applicationInfo.nativeLibraryDir,
                collectNetworkSnapshot(context),
                tunFd,
                protector
            ) as? String ?: ""
            initialized = true
            lastError = null
            result
        } catch (e: Throwable) {
            initialized = false
            fail(unwrap(e))
        }
    }

    fun updateNetwork(context: Context): Boolean {
        if (!initialized) return false
        return try {
            val method = mobileClass().getMethod("updatePlatformNetwork", String::class.java)
            val raw = method.invoke(null, collectNetworkSnapshot(context)) as? String ?: return false
            raw.contains("\"ok\":true")
        } catch (_: Throwable) {
            false
        }
    }

    private fun collectNetworkSnapshot(context: Context): String {
        return try {
            val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
            val dnsByInterface = HashMap<String, MutableList<String>>()
            val typeByInterface = HashMap<String, Int>()
            val meteredByInterface = HashMap<String, Boolean>()
            var defaultName: String? = null
            var bestScore = -1
            for (network in cm.allNetworks) {
                val caps = cm.getNetworkCapabilities(network) ?: continue
                if (!caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN) ||
                    !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)) continue
                val link = cm.getLinkProperties(network) ?: continue
                val name = link.interfaceName ?: continue
                if (name.startsWith("tun") || name.startsWith("ipsec")) continue
                var score = 1
                if (caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)) score += 4
                if (network == cm.activeNetwork) score += 2
                if (caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)) score += 3
                if (score > bestScore) {
                    bestScore = score
                    defaultName = name
                }
                val dns = dnsByInterface.getOrPut(name) { mutableListOf() }
                for (addr in link.dnsServers) {
                    addr.hostAddress?.let { if (!dns.contains(it)) dns.add(it) }
                }
                typeByInterface[name] = when {
                    caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> 1
                    caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> 2
                    caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> 3
                    else -> 4
                }
                meteredByInterface[name] = !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
            }
            val arr = JSONArray()
            var defaultIndex = 0
            val interfaces = NetworkInterface.getNetworkInterfaces()
            if (interfaces != null) {
                for (nif in Collections.list(interfaces)) {
                    if (!nif.isUp || nif.isLoopback) continue
                    val name = nif.name ?: continue
                    if (name.startsWith("tun") || name.startsWith("ipsec")) continue
                    val addresses = JSONArray()
                    for (ia in nif.interfaceAddresses) {
                        val host = ia.address?.hostAddress?.substringBefore('%') ?: continue
                        addresses.put("$host/${ia.networkPrefixLength}")
                    }
                    if (addresses.length() == 0) continue
                    if (defaultName == null) defaultName = name
                    if (name == defaultName) defaultIndex = nif.index
                    var flags = 1 or 32
                    if (nif.isPointToPoint) flags = flags or 8
                    if (nif.supportsMulticast()) flags = flags or 16
                    val item = JSONObject()
                    item.put("name", name)
                    item.put("index", nif.index)
                    item.put("mtu", nif.mtu)
                    item.put("flags", flags)
                    item.put("addresses", addresses)
                    item.put("dns", JSONArray(dnsByInterface[name] ?: emptyList<String>()))
                    item.put("type", typeByInterface[name] ?: 4)
                    item.put("metered", meteredByInterface[name] ?: false)
                    arr.put(item)
                }
            }
            val root = JSONObject()
            root.put("default", defaultName ?: "")
            root.put("default_index", defaultIndex)
            root.put("interfaces", arr)
            root.toString()
        } catch (_: Throwable) {
            "{}"
        }
    }
    fun connect(engineMode: String, countryCode: String, batterySaver: Boolean): String =
        invoke("connect", engineMode, countryCode, batterySaver)

    fun disconnect(): String = invoke("disconnect")

    fun status(): String = invoke("getStatusJSON")

    fun testIpHealth(): String = invoke("testIPHealth")

    fun runDiagnostics(): String = invoke("runDiagnostics")

    fun toggleLan(enabled: Boolean, usePin: Boolean): String =
        invoke("toggleLANSharing", enabled, usePin)

    fun checkUpdate(): String = invoke("checkAppUpdate")

    fun downloadUpdate(cacheDir: String, url: String, sha256: String): String =
        invoke("downloadAppUpdate", cacheDir, url, sha256)

    fun checkUpdateStandalone(context: Context): String {
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        @Suppress("DEPRECATION")
        val rawBuild = if (android.os.Build.VERSION.SDK_INT >= 28) {
            info.longVersionCode.toInt()
        } else {
            info.versionCode
        }
        // Flutter's --split-per-abi adds 1000/2000 to the versionCode. The
        // signed manifest uses the pubspec build number, so compare that base.
        val build = rawBuild % 1000
        return invoke("checkAppUpdateStandalone", build, info.versionName ?: "1.0.0")
    }

    private fun invoke(name: String, vararg args: Any?): String {
        val cls = mobileClass()
        val types = args.map {
            when (it) {
                is Boolean -> Boolean::class.javaPrimitiveType
                is Int -> Int::class.javaPrimitiveType
                is String -> String::class.java
                else -> it?.javaClass
            }
        }.toTypedArray()
        return try {
            val method = cls.getMethod(name, *types)
            method.invoke(null, *args) as? String ?: ""
        } catch (e: Throwable) {
            fail(unwrap(e))
        }
    }

    private fun unwrap(e: Throwable): String {
        val root = if (e is InvocationTargetException) (e.targetException ?: e) else e
        return root.message?.takeIf { it.isNotBlank() }
            ?: "عملیات هسته اندروید ناموفق بود (${root.javaClass.simpleName})."
    }
}
