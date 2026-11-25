import 'dart:convert';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/vpn_models.dart';
import 'engine_bridge.dart';

/// Android bridge communicating with Kotlin MlzVpnService (which hosts mlz-core.aar).
class AndroidEngineBridge implements EngineBridge {
  static const _channel = MethodChannel('com.mlzvpn.app/engine');

  Future<Map<String, dynamic>> _invokeJson(String method, [Map<String, dynamic>? args]) async {
    final raw = await _channel.invokeMethod<String>(method, args);
    if (raw == null || raw.isEmpty) return <String, dynamic>{};
    return jsonDecode(raw) as Map<String, dynamic>;
  }

  @override
  Future<Map<String, dynamic>> connect({
    required TunnelModeOption tunnelMode,
    required EngineModeOption engineMode,
    required CountryChoice country,
    required bool batterySaver,
  }) {
    return _invokeJson('connect', {
      'engine_mode': engineMode.wireValue,
      'country_code': country.isAuto ? 'auto' : country.code,
      'battery_saver': batterySaver,
    });
  }

  @override
  Future<Map<String, dynamic>> disconnect() => _invokeJson('disconnect');

  @override
  Future<Map<String, dynamic>> getStatus() => _invokeJson('getStatus');

  @override
  Future<IpTestResultModel> testIpHealth() async {
    final res = await _invokeJson('testIpHealth');
    return IpTestResultModel.fromJson(res);
  }

  @override
  Future<DiagnosticsReportModel> runDiagnostics() async {
    final res = await _invokeJson('runDiagnostics');
    return DiagnosticsReportModel.fromJson(res);
  }

  @override
  Future<LanShareModel> toggleLanShare({required bool enabled, required bool usePin}) async {
    final res = await _invokeJson('toggleLan', {'enabled': enabled, 'use_pin': usePin});
    return LanShareModel.fromJson(res);
  }

  @override
  Future<AppUpdateInfo?> checkUpdate() async {
    final res = await _invokeJson('checkUpdate');
    if ((res['update_available'] as bool?) != true) return null;
    final m = (res['manifest'] as Map<String, dynamic>?) ?? const {};
    final key = (res['asset_key'] as String?) ?? 'android_arm64';
    final assets = (m['assets'] as Map<String, dynamic>?) ?? const {};
    final asset = (assets[key] as Map<String, dynamic>?) ?? const {};
    return AppUpdateInfo(
      updateAvailable: true,
      version: (m['version'] as String?) ?? '',
      changelogFA: ((m['changelog_fa'] as List<dynamic>?) ?? const []).map((e) => e.toString()).toList(),
      downloadUrl: (asset['url'] as String?) ?? '',
      sha256: (asset['sha256'] as String?) ?? '',
    );
  }

  @override
  Future<void> downloadAndInstallUpdate(AppUpdateInfo info) async {
    if (info.downloadUrl.isEmpty) return;
    await launchUrl(Uri.parse(info.downloadUrl), mode: LaunchMode.externalApplication);
  }

  @override
  Future<void> dispose() async {}
}
