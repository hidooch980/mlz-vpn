import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'dart:math';

import 'package:ffi/ffi.dart';
import 'package:path/path.dart' as p;
import 'package:url_launcher/url_launcher.dart';

import '../models/vpn_models.dart';
import 'engine_bridge.dart';

typedef _ShellExecuteWNative = IntPtr Function(
  IntPtr hwnd,
  Pointer<Utf16> lpOperation,
  Pointer<Utf16> lpFile,
  Pointer<Utf16> lpParameters,
  Pointer<Utf16> lpDirectory,
  Int32 nShowCmd,
);
typedef _ShellExecuteWDart = int Function(
  int hwnd,
  Pointer<Utf16> lpOperation,
  Pointer<Utf16> lpFile,
  Pointer<Utf16> lpParameters,
  Pointer<Utf16> lpDirectory,
  int nShowCmd,
);

/// Windows bridge implementing the user's choice:
/// - Proxy mode runs unprivileged without UAC prompt.
/// - TUN mode triggers Windows UAC ("runas") ONLY on Connect and shuts down cleanly on Disconnect.
class WindowsEngineBridge implements EngineBridge {
  Process? _unprivilegedProc;
  int? _activePort;
  String? _sessionToken;
  bool _isElevatedSession = false;

  Future<void> _ensureStarted(TunnelModeOption tunnelMode) async {
    final wantsElevated = tunnelMode == TunnelModeOption.tun;
    if (_activePort != null && _isElevatedSession == wantsElevated) {
      try {
        await _request('GET', '/v1/status');
        return;
      } catch (_) {
        await _shutdownCurrent();
      }
    }
    await _shutdownCurrent();
    // Give the previous engine a moment to release the single-instance lock.
    await Future<void>.delayed(const Duration(milliseconds: 350));

    final port = await _allocateFreePort();
    final token = _generateToken();
    final exePath = _resolveEngineExecutable();
    // The installed layout puts the rules next to the executable. Pass the
    // absolute path so a Start-menu shortcut does not depend on its working
    // directory.
    final rulesDir = p.join(File(exePath).parent.path, 'rules');
    final args =
        '--mode=${tunnelMode.wireValue} --ipc-port=$port --token=$token --ruleset-dir="$rulesDir"';

    if (wantsElevated) {
      _launchElevatedRunAs(exePath, args);
    } else {
      _unprivilegedProc = await Process.start(
        exePath,
        [
          '--mode=proxy',
          '--ipc-port=$port',
          '--token=$token',
          '--ruleset-dir=$rulesDir',
        ],
        mode: ProcessStartMode.detachedWithStdio,
      );
    }

    _activePort = port;
    _sessionToken = token;
    _isElevatedSession = wantsElevated;

    // Wait up to 8 seconds for mlz-engine IPC readiness.
    for (var i = 0; i < 40; i++) {
      try {
        await _request('GET', '/v1/status');
        return;
      } catch (_) {
        await Future<void>.delayed(const Duration(milliseconds: 200));
      }
    }
    throw Exception('راه‌اندازی هسته ویندوز با تاخیر یا لغو مجوز مواجه شد.');
  }

  void _launchElevatedRunAs(String exePath, String arguments) {
    final shell32 = DynamicLibrary.open('shell32.dll');
    final shellExec =
        shell32.lookupFunction<_ShellExecuteWNative, _ShellExecuteWDart>(
            'ShellExecuteW');

    final op = 'runas'.toNativeUtf16();
    final file = exePath.toNativeUtf16();
    final params = arguments.toNativeUtf16();
    final dir = File(exePath).parent.path.toNativeUtf16();
    try {
      const swHide = 0;
      final res = shellExec(0, op, file, params, dir, swHide);
      if (res <= 32) {
        throw Exception(
            'درخواست دسترسی ادمین (UAC) رد شد یا اجرا نشد (کد: $res).');
      }
    } finally {
      calloc.free(op);
      calloc.free(file);
      calloc.free(params);
      calloc.free(dir);
    }
  }

  String _resolveEngineExecutable() {
    final appDir = File(Platform.resolvedExecutable).parent.path;
    final adjacent = p.join(appDir, 'mlz-engine.exe');
    if (File(adjacent).existsSync()) return adjacent;
    return p.join(Directory.current.path, 'engine', 'bin', 'mlz-engine.exe');
  }

  Future<int> _allocateFreePort() async {
    final srv = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final port = srv.port;
    await srv.close();
    return port;
  }

  String _generateToken() {
    final rng = Random.secure();
    return List<String>.generate(
        32, (_) => rng.nextInt(256).toRadixString(16).padLeft(2, '0')).join();
  }

  Future<Map<String, dynamic>> _request(String method, String path,
      [Map<String, dynamic>? body]) async {
    if (_activePort == null) {
      throw Exception('Engine is not running');
    }
    final client = HttpClient()..connectionTimeout = const Duration(seconds: 4);
    try {
      final uri = Uri.http('127.0.0.1:$_activePort', path);
      final req = await client.openUrl(method, uri);
      if (_sessionToken != null) {
        req.headers
            .set(HttpHeaders.authorizationHeader, 'Bearer $_sessionToken');
      }
      if (body != null) {
        req.headers.contentType = ContentType.json;
        req.write(jsonEncode(body));
      }
      final resp = await req.close();
      final text = await utf8.decodeStream(resp);
      final decoded = text.trim().isEmpty
          ? <String, dynamic>{}
          : jsonDecode(text) as Map<String, dynamic>;
      if (resp.statusCode >= 400) {
        throw Exception(
            (decoded['error'] ?? 'HTTP ${resp.statusCode}').toString());
      }
      return decoded;
    } finally {
      client.close(force: true);
    }
  }

  Future<void> _shutdownCurrent() async {
    if (_activePort != null) {
      try {
        await _request('POST', '/v1/shutdown');
      } catch (_) {}
    }
    _unprivilegedProc?.kill();
    _unprivilegedProc = null;
    _activePort = null;
    _sessionToken = null;
    _isElevatedSession = false;
  }

  @override
  Future<Map<String, dynamic>> connect({
    required TunnelModeOption tunnelMode,
    required EngineModeOption engineMode,
    required CountryChoice country,
    required bool batterySaver,
  }) async {
    await _ensureStarted(tunnelMode);
    return _request('POST', '/v1/connect', {
      'tunnel_mode': tunnelMode.wireValue,
      'engine_mode': engineMode.wireValue,
      'country': {
        'auto': country.isAuto,
        'code': country.isAuto ? '' : country.code
      },
      'battery_saver': batterySaver,
    });
  }

  @override
  Future<Map<String, dynamic>> disconnect() async {
    if (_activePort == null) {
      return {'state': 'disconnected'};
    }
    try {
      await _request('POST', '/v1/disconnect');
    } finally {
      // Elevated TUN process closes immediately on disconnect so no background admin process persists.
      if (_isElevatedSession) {
        await _shutdownCurrent();
      }
    }
    return {'state': 'disconnected'};
  }

  @override
  Future<Map<String, dynamic>> getStatus() async {
    if (_activePort == null) return {'state': 'disconnected'};
    return _request('GET', '/v1/status');
  }

  @override
  Future<IpTestResultModel> testIpHealth() async {
    if (_activePort == null) {
      await _ensureStarted(TunnelModeOption.proxy);
    }
    final res = await _request('POST', '/v1/test-ip');
    return IpTestResultModel.fromJson(res);
  }

  @override
  Future<DiagnosticsReportModel> runDiagnostics() async {
    if (_activePort == null) {
      await _ensureStarted(TunnelModeOption.proxy);
    }
    final res = await _request('POST', '/v1/diagnostics');
    return DiagnosticsReportModel.fromJson(res);
  }

  @override
  Future<DiagnosticsReportModel> autoHealDiagnostics({
    required TunnelModeOption tunnelMode,
  }) async {
    await _ensureStarted(tunnelMode);
    final res = await _request('POST', '/v1/diagnostics/heal', {
      'tunnel_mode': tunnelMode == TunnelModeOption.tun ? 'tun' : 'proxy',
    });
    return DiagnosticsReportModel.fromJson(res);
  }

  @override
  Future<LanShareModel> toggleLanShare(
      {required bool enabled, required bool usePin}) async {
    if (_activePort == null) {
      await _ensureStarted(TunnelModeOption.proxy);
    }
    final res = await _request(
        'POST', '/v1/lan', {'enabled': enabled, 'use_pin': usePin});
    return LanShareModel.fromJson(res);
  }

  @override
  Future<AppUpdateInfo?> checkUpdate() async {
    if (_activePort == null) {
      await _ensureStarted(TunnelModeOption.proxy);
    }
    final res = await _request('GET', '/v1/updates');
    final available = (res['update_available'] as bool?) ?? false;
    final manifest = res['manifest'] as Map<String, dynamic>?;
    if (!available || manifest == null) return null;

    final assetKey = (res['asset_key'] as String?) ?? 'windows_x64';
    final assets = (manifest['assets'] as Map<String, dynamic>?) ?? const {};
    final asset = (assets[assetKey] as Map<String, dynamic>?) ?? const {};
    return AppUpdateInfo(
      updateAvailable: true,
      version: (manifest['version'] as String?) ?? '1.0.0',
      changelogFA: ((manifest['changelog_fa'] as List<dynamic>?) ?? const [])
          .map((e) => e.toString())
          .toList(),
      downloadUrl: (asset['url'] as String?) ?? '',
      sha256: (asset['sha256'] as String?) ?? '',
    );
  }

  @override
  Future<void> downloadAndInstallUpdate(AppUpdateInfo info) async {
    if (info.downloadUrl.isEmpty || info.sha256.isEmpty) {
      throw Exception('اطلاعات به‌روزرسانی ناقص است؛ دوباره بررسی کنید.');
    }
    if (_activePort == null) {
      await _ensureStarted(TunnelModeOption.proxy);
    }
    final isSetup = info.downloadUrl.toLowerCase().endsWith('.exe');
    final res = await _request('POST', '/v1/download-update', {
      'url': info.downloadUrl,
      'sha256': info.sha256,
      'file_name': isSetup
          ? 'mlz-vpn-setup-${info.version}.exe'
          : 'mlz-vpn-update-${info.version}.zip',
    });
    final path = res['path'] as String?;
    if (path == null || path.isEmpty) return;
    if (isSetup) {
      // Per-user NSIS installer needs no UAC; it closes the running app, replaces
      // files, and relaunches the new build when the user opens it again.
      await Process.start(path, const [], mode: ProcessStartMode.detached);
    } else {
      await launchUrl(Uri.file(File(path).parent.path));
    }
  }

  @override
  Future<void> dispose() => _shutdownCurrent();
}
