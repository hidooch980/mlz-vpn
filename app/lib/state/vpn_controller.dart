import 'dart:async';
import 'package:material_ui/material_ui.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../models/vpn_models.dart';
import '../services/engine_bridge.dart';

class MlzAppState {
  final VpnConnectionState connState;
  final ThemeMode themeMode;
  final TunnelModeOption tunnelMode;
  final EngineModeOption engineMode;
  final CountryChoice country;
  final bool batterySaver;
  final bool appLockEnabled;
  final bool isUnlocked;
  final int realDelayMs;
  final int uploadBps;
  final int downloadBps;
  final int uploadTotalBytes;
  final int downloadTotalBytes;
  final int connectedSeconds;
  final bool ipTesting;
  final IpTestResultModel? lastIpTest;
  final DiagnosticsReportModel? diagnosticsReport;
  final bool diagnosticsRunning;
  final LanShareModel lanShare;
  final AppUpdateInfo? availableUpdate;
  final String? errorMessageFA;

  const MlzAppState({
    this.connState = VpnConnectionState.disconnected,
    this.themeMode = ThemeMode.system,
    this.tunnelMode = TunnelModeOption.tun,
    this.engineMode = EngineModeOption.auto,
    this.country = const CountryChoice(code: 'auto', nameFA: 'اتوماتیک (بهترین کشور)', flag: '🌐', hintPing: 'خودکار'),
    this.batterySaver = true,
    this.appLockEnabled = false,
    this.isUnlocked = true,
    this.realDelayMs = 0,
    this.uploadBps = 0,
    this.downloadBps = 0,
    this.uploadTotalBytes = 0,
    this.downloadTotalBytes = 0,
    this.connectedSeconds = 0,
    this.ipTesting = false,
    this.lastIpTest,
    this.diagnosticsReport,
    this.diagnosticsRunning = false,
    this.lanShare = const LanShareModel(
      enabled: false,
      primaryIp: '192.168.137.1',
      port: 10808,
      username: '',
      pin: '',
      firewallManaged: false,
    ),
    this.availableUpdate,
    this.errorMessageFA,
  });

  MlzAppState copyWith({
    VpnConnectionState? connState,
    ThemeMode? themeMode,
    TunnelModeOption? tunnelMode,
    EngineModeOption? engineMode,
    CountryChoice? country,
    bool? batterySaver,
    bool? appLockEnabled,
    bool? isUnlocked,
    int? realDelayMs,
    int? uploadBps,
    int? downloadBps,
    int? uploadTotalBytes,
    int? downloadTotalBytes,
    int? connectedSeconds,
    bool? ipTesting,
    IpTestResultModel? lastIpTest,
    DiagnosticsReportModel? diagnosticsReport,
    bool? diagnosticsRunning,
    LanShareModel? lanShare,
    AppUpdateInfo? availableUpdate,
    String? errorMessageFA,
  }) {
    return MlzAppState(
      connState: connState ?? this.connState,
      themeMode: themeMode ?? this.themeMode,
      tunnelMode: tunnelMode ?? this.tunnelMode,
      engineMode: engineMode ?? this.engineMode,
      country: country ?? this.country,
      batterySaver: batterySaver ?? this.batterySaver,
      appLockEnabled: appLockEnabled ?? this.appLockEnabled,
      isUnlocked: isUnlocked ?? this.isUnlocked,
      realDelayMs: realDelayMs ?? this.realDelayMs,
      uploadBps: uploadBps ?? this.uploadBps,
      downloadBps: downloadBps ?? this.downloadBps,
      uploadTotalBytes: uploadTotalBytes ?? this.uploadTotalBytes,
      downloadTotalBytes: downloadTotalBytes ?? this.downloadTotalBytes,
      connectedSeconds: connectedSeconds ?? this.connectedSeconds,
      ipTesting: ipTesting ?? this.ipTesting,
      lastIpTest: lastIpTest ?? this.lastIpTest,
      diagnosticsReport: diagnosticsReport ?? this.diagnosticsReport,
      diagnosticsRunning: diagnosticsRunning ?? this.diagnosticsRunning,
      lanShare: lanShare ?? this.lanShare,
      availableUpdate: availableUpdate ?? this.availableUpdate,
      errorMessageFA: errorMessageFA,
    );
  }
}

final vpnControllerProvider = StateNotifierProvider<VpnController, MlzAppState>((ref) {
  return VpnController(EngineBridge.create());
});

class VpnController extends StateNotifier<MlzAppState> {
  final EngineBridge _bridge;
  Timer? _pollTimer;

  VpnController(this._bridge) : super(const MlzAppState()) {
    _loadPrefs();
  }

  Future<void> _loadPrefs() async {
    final prefs = await SharedPreferences.getInstance();
    final themeIndex = prefs.getInt('theme_mode') ?? 0;
    final appLock = prefs.getBool('app_lock') ?? false;
    final battery = prefs.getBool('battery_saver') ?? true;
    final tunnelIdx = prefs.getInt('tunnel_mode') ?? 0;

    state = state.copyWith(
      themeMode: ThemeMode.values[themeIndex.clamp(0, 2)],
      appLockEnabled: appLock,
      isUnlocked: !appLock,
      batterySaver: battery,
      tunnelMode: TunnelModeOption.values[tunnelIdx.clamp(0, 1)],
    );
  }

  Future<void> toggleConnection() async {
    if (state.connState == VpnConnectionState.connected ||
        state.connState == VpnConnectionState.connecting ||
        state.connState == VpnConnectionState.healing) {
      await _disconnect();
      return;
    }

    state = state.copyWith(connState: VpnConnectionState.connecting, errorMessageFA: null);
    try {
      final snap = await _bridge.connect(
        tunnelMode: state.tunnelMode,
        engineMode: state.engineMode,
        country: state.country,
        batterySaver: state.batterySaver,
      );
      _applySnapshot(snap);
      _startTelemetryPoll();
    } catch (e) {
      state = state.copyWith(
        connState: VpnConnectionState.error,
        errorMessageFA: e.toString().replaceFirst('Exception: ', ''),
      );
    }
  }

  Future<void> _disconnect() async {
    _pollTimer?.cancel();
    _pollTimer = null;
    try {
      await _bridge.disconnect();
    } finally {
      state = state.copyWith(
        connState: VpnConnectionState.disconnected,
        connectedSeconds: 0,
        uploadBps: 0,
        downloadBps: 0,
      );
    }
  }

  void _startTelemetryPoll() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 1), (_) async {
      if (state.connState == VpnConnectionState.connected) {
        state = state.copyWith(connectedSeconds: state.connectedSeconds + 1);
      }
      try {
        final snap = await _bridge.getStatus();
        _applySnapshot(snap);
      } catch (_) {}
    });
  }

  void _applySnapshot(Map<String, dynamic> snap) {
    final st = VpnConnectionState.fromWire(snap['state'] as String?);
    final stats = (snap['stats'] as Map<String, dynamic>?) ?? const {};
    final lanRaw = snap['lan'] as Map<String, dynamic>?;
    final ipRaw = snap['last_ip_test'] as Map<String, dynamic>?;

    state = state.copyWith(
      connState: st,
      realDelayMs: (stats['real_delay_ms'] as num?)?.toInt() ?? state.realDelayMs,
      uploadBps: (stats['upload_bps'] as num?)?.toInt() ?? 0,
      downloadBps: (stats['download_bps'] as num?)?.toInt() ?? 0,
      uploadTotalBytes: (stats['upload_total_bytes'] as num?)?.toInt() ?? state.uploadTotalBytes,
      downloadTotalBytes: (stats['download_total_bytes'] as num?)?.toInt() ?? state.downloadTotalBytes,
      lanShare: lanRaw != null ? LanShareModel.fromJson(lanRaw) : state.lanShare,
      lastIpTest: ipRaw != null ? IpTestResultModel.fromJson(ipRaw) : state.lastIpTest,
      errorMessageFA: snap['error_message_fa'] as String?,
    );
  }

  /// Triggered when the user clicks on the IP display rectangle.
  Future<void> testIpRectangle() async {
    if (state.ipTesting) return;
    state = state.copyWith(ipTesting: true);
    try {
      final res = await _bridge.testIpHealth();
      state = state.copyWith(
        ipTesting: false,
        lastIpTest: res,
        realDelayMs: res.realDelayMs > 0 ? res.realDelayMs : state.realDelayMs,
      );
    } catch (e) {
      state = state.copyWith(
        ipTesting: false,
        errorMessageFA: 'تست آی‌پی ناموفق بود: ${e.toString().replaceFirst('Exception: ', '')}',
      );
    }
  }

  Future<void> selectCountry(CountryChoice choice) async {
    state = state.copyWith(country: choice);
    if (state.connState == VpnConnectionState.connected) {
      await _disconnect();
      await toggleConnection();
    }
  }

  Future<void> selectEngineMode(EngineModeOption mode) async {
    state = state.copyWith(engineMode: mode);
    if (state.connState == VpnConnectionState.connected) {
      await _disconnect();
      await toggleConnection();
    }
  }

  Future<void> setThemeMode(ThemeMode mode) async {
    state = state.copyWith(themeMode: mode);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt('theme_mode', mode.index);
  }

  Future<void> setTunnelMode(TunnelModeOption mode) async {
    state = state.copyWith(tunnelMode: mode);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt('tunnel_mode', mode.index);
  }

  Future<void> setBatterySaver(bool enabled) async {
    state = state.copyWith(batterySaver: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('battery_saver', enabled);
  }

  Future<void> setAppLock(bool enabled) async {
    state = state.copyWith(appLockEnabled: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('app_lock', enabled);
  }

  void markUnlocked() => state = state.copyWith(isUnlocked: true);

  Future<void> runDiagnostics() async {
    state = state.copyWith(diagnosticsRunning: true);
    try {
      final report = await _bridge.runDiagnostics();
      state = state.copyWith(diagnosticsRunning: false, diagnosticsReport: report);
    } catch (_) {
      state = state.copyWith(diagnosticsRunning: false);
    }
  }

  Future<void> toggleLanSharing({required bool enabled, required bool usePin}) async {
    try {
      final info = await _bridge.toggleLanShare(enabled: enabled, usePin: usePin);
      state = state.copyWith(lanShare: info);
    } catch (_) {}
  }

  Future<void> checkForUpdates() async {
    try {
      final info = await _bridge.checkUpdate();
      state = state.copyWith(availableUpdate: info);
    } catch (_) {}
  }

  Future<void> downloadAndInstallUpdate() async {
    final upd = state.availableUpdate;
    if (upd == null || !upd.updateAvailable) return;
    try {
      await _bridge.downloadAndInstallUpdate(upd);
    } catch (e) {
      state = state.copyWith(
        errorMessageFA: 'خطا در دریافت به‌روزرسانی: ${e.toString().replaceFirst('Exception: ', '')}',
      );
    }
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _bridge.dispose();
    super.dispose();
  }
}
