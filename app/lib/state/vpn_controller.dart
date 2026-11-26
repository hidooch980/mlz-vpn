import 'dart:async';
import 'package:material_ui/material_ui.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';
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
  final bool diagnosticsHealing;
  final bool lanBusy;
  final bool updateChecking;
  final bool updateDownloading;
  final LanShareModel lanShare;
  final AppUpdateInfo? availableUpdate;
  final String appVersion;
  final String? errorMessageFA;

  const MlzAppState({
    this.connState = VpnConnectionState.disconnected,
    this.themeMode = ThemeMode.system,
    this.tunnelMode = TunnelModeOption.tun,
    this.engineMode = EngineModeOption.auto,
    this.country = const CountryChoice(
        code: 'auto',
        nameFA: 'اتوماتیک (بهترین کشور)',
        flag: '🌐',
        hintPing: 'خودکار'),
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
    this.diagnosticsHealing = false,
    this.lanBusy = false,
    this.updateChecking = false,
    this.updateDownloading = false,
    this.lanShare = const LanShareModel(
      enabled: false,
      primaryIp: '192.168.137.1',
      port: 10808,
      username: '',
      pin: '',
      firewallManaged: false,
    ),
    this.availableUpdate,
    this.appVersion = '1.0.0',
    this.errorMessageFA,
  });

  static const Object _unset = Object();

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
    Object? lastIpTest = _unset,
    Object? diagnosticsReport = _unset,
    bool? diagnosticsRunning,
    bool? diagnosticsHealing,
    bool? lanBusy,
    bool? updateChecking,
    bool? updateDownloading,
    LanShareModel? lanShare,
    Object? availableUpdate = _unset,
    String? appVersion,
    Object? errorMessageFA = _unset,
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
      lastIpTest: identical(lastIpTest, _unset)
          ? this.lastIpTest
          : lastIpTest as IpTestResultModel?,
      diagnosticsReport: identical(diagnosticsReport, _unset)
          ? this.diagnosticsReport
          : diagnosticsReport as DiagnosticsReportModel?,
      diagnosticsRunning: diagnosticsRunning ?? this.diagnosticsRunning,
      diagnosticsHealing: diagnosticsHealing ?? this.diagnosticsHealing,
      lanBusy: lanBusy ?? this.lanBusy,
      updateChecking: updateChecking ?? this.updateChecking,
      updateDownloading: updateDownloading ?? this.updateDownloading,
      lanShare: lanShare ?? this.lanShare,
      availableUpdate: identical(availableUpdate, _unset)
          ? this.availableUpdate
          : availableUpdate as AppUpdateInfo?,
      appVersion: appVersion ?? this.appVersion,
      errorMessageFA: identical(errorMessageFA, _unset)
          ? this.errorMessageFA
          : errorMessageFA as String?,
    );
  }
}

final vpnControllerProvider =
    StateNotifierProvider<VpnController, MlzAppState>((ref) {
  return VpnController(EngineBridge.create());
});

/// QA-only: simulate an Iranian exit so the IP card and .ir checks can be
/// exercised without a real Iranian node. Enable with
/// `--dart-define=MLZ_TEST_IRAN_MOCK=true`; release builds leave it false.
const bool kQaIranMock =
    bool.fromEnvironment('MLZ_TEST_IRAN_MOCK', defaultValue: false);

class VpnController extends StateNotifier<MlzAppState> {
  final EngineBridge _bridge;
  Timer? _pollTimer;
  bool _ipAutoTested = false;

  VpnController(this._bridge) : super(const MlzAppState()) {
    _loadPrefs();
  }

  Future<void> _loadPrefs() async {
    final prefs = await SharedPreferences.getInstance();
    final themeIndex = prefs.getInt('theme_mode') ?? 0;
    final appLock = prefs.getBool('app_lock') ?? false;
    final battery = prefs.getBool('battery_saver') ?? true;
    final tunnelIdx = prefs.getInt('tunnel_mode') ?? 0;
    String version = '1.0.0';
    try {
      final info = await PackageInfo.fromPlatform();
      if (info.version.isNotEmpty) version = info.version;
    } catch (_) {}

    state = state.copyWith(
      themeMode: ThemeMode.values[themeIndex.clamp(0, 2)],
      appLockEnabled: appLock,
      isUnlocked: !appLock,
      batterySaver: battery,
      tunnelMode: TunnelModeOption.values[tunnelIdx.clamp(0, 1)],
      appVersion: version,
    );
    unawaited(_checkUpdateOnStartup());
  }

  Future<void> _checkUpdateOnStartup() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final last = prefs.getInt('last_update_check_ms') ?? 0;
      final now = DateTime.now().millisecondsSinceEpoch;
      if (now - last < const Duration(minutes: 10).inMilliseconds) return;
    } catch (_) {}
    await checkForUpdates(silent: true);
  }

  Future<void> toggleConnection() async {
    if (state.connState == VpnConnectionState.connected ||
        state.connState == VpnConnectionState.connecting ||
        state.connState == VpnConnectionState.healing) {
      await _disconnect();
      return;
    }

    _ipAutoTested = false;
    state = state.copyWith(
        connState: VpnConnectionState.connecting,
        errorMessageFA: null,
        lastIpTest: null);
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
      _ipAutoTested = false;
      state = state.copyWith(
        connState: VpnConnectionState.error,
        errorMessageFA: _cleanError(e),
        lastIpTest: null,
      );
      // Automatic invisible diagnosis after a failed connection: the report is
      // ready when the user opens the diagnostics screen, without hiding the
      // original connection error.
      unawaited(_diagnoseAfterFailure());
    }
  }

  Future<void> _diagnoseAfterFailure() async {
    if (state.diagnosticsRunning || state.diagnosticsHealing) return;
    state = state.copyWith(diagnosticsRunning: true);
    try {
      final report =
          await _bridge.runDiagnostics().timeout(const Duration(seconds: 100));
      state = state.copyWith(
        diagnosticsRunning: false,
        diagnosticsReport: report,
      );
    } catch (_) {
      state = state.copyWith(diagnosticsRunning: false);
    }
  }

  Future<void> _disconnect() async {
    _pollTimer?.cancel();
    _pollTimer = null;
    try {
      await _bridge.disconnect();
    } finally {
      _ipAutoTested = false;
      state = state.copyWith(
        connState: VpnConnectionState.disconnected,
        lastIpTest: null,
        connectedSeconds: 0,
        uploadBps: 0,
        downloadBps: 0,
        errorMessageFA: null,
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
    final rawServerError = (snap['error_message_fa'] as String?)?.trim();
    final serverError = (rawServerError == null || rawServerError.isEmpty)
        ? rawServerError
        : _cleanError(rawServerError);
    final keepError = st == VpnConnectionState.error &&
        (serverError == null || serverError.isEmpty);

    state = state.copyWith(
      connState: st,
      realDelayMs:
          (stats['real_delay_ms'] as num?)?.toInt() ?? state.realDelayMs,
      uploadBps: (stats['upload_bps'] as num?)?.toInt() ?? 0,
      downloadBps: (stats['download_bps'] as num?)?.toInt() ?? 0,
      uploadTotalBytes: (stats['upload_total_bytes'] as num?)?.toInt() ??
          state.uploadTotalBytes,
      downloadTotalBytes: (stats['download_total_bytes'] as num?)?.toInt() ??
          state.downloadTotalBytes,
      lanShare:
          lanRaw != null ? LanShareModel.fromJson(lanRaw) : state.lanShare,
      lastIpTest:
          ipRaw != null ? IpTestResultModel.fromJson(ipRaw) : state.lastIpTest,
      errorMessageFA: keepError
          ? state.errorMessageFA
          : (serverError == null || serverError.isEmpty ? null : serverError),
    );
    if (st == VpnConnectionState.connected &&
        !_ipAutoTested &&
        !state.ipTesting) {
      _ipAutoTested = true;
      unawaited(testIpRectangle());
    }
  }

  /// Triggered when the user clicks on the IP display rectangle.
  Future<void> testIpRectangle() async {
    if (state.ipTesting) return;
    state = state.copyWith(ipTesting: true);
    try {
      final IpTestResultModel res;
      if (kQaIranMock) {
        await Future<void>.delayed(const Duration(milliseconds: 700));
        res = const IpTestResultModel(
          publicIp: '5.57.38.194',
          countryCode: 'IR',
          countryNameFA: 'ایران (شبیه‌سازی تست)',
          realDelayMs: 38,
          dnsProtected: true,
          iranDirectOk: true,
        );
      } else {
        res = await _bridge.testIpHealth().timeout(const Duration(seconds: 20));
      }
      state = state.copyWith(
        ipTesting: false,
        lastIpTest: res,
        realDelayMs: res.realDelayMs > 0 ? res.realDelayMs : state.realDelayMs,
      );
    } catch (e) {
      state = state.copyWith(
        ipTesting: false,
        errorMessageFA: e is TimeoutException
            ? 'تست آی‌پی بیش از حد طول کشید؛ دوباره تلاش کنید.'
            : 'تست آی‌پی ناموفق بود: ${_cleanError(e)}',
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

  void clearError() => state = state.copyWith(errorMessageFA: null);

  /// Re-enters the real platform connection path from the diagnostics screen.
  /// Calling the Go auto-healer while disconnected cannot recreate Android's
  /// VpnService TUN, so a failed attempt must be retried through the normal
  /// connect flow. The method waits for the asynchronous Android service, then
  /// refreshes the report with the outcome.
  Future<void> retryConnectionAndDiagnose() async {
    if (state.connState == VpnConnectionState.connecting ||
        state.connState == VpnConnectionState.healing ||
        state.diagnosticsRunning ||
        state.diagnosticsHealing) {
      return;
    }
    await toggleConnection();
    final deadline = DateTime.now().add(const Duration(seconds: 150));
    while (DateTime.now().isBefore(deadline)) {
      await Future<void>.delayed(const Duration(milliseconds: 900));
      await _refreshStatusSnapshot();
      if (state.connState == VpnConnectionState.connected ||
          state.connState == VpnConnectionState.error ||
          (state.connState == VpnConnectionState.disconnected &&
              (state.errorMessageFA ?? '').isNotEmpty)) {
        break;
      }
    }
    if (state.connState == VpnConnectionState.connecting) {
      state = state.copyWith(
        connState: VpnConnectionState.error,
        errorMessageFA:
            'اتصال در زمان مورد انتظار برقرار نشد؛ چند لحظه بعد دوباره تلاش کنید.',
      );
    }
    // The engine triggers the IP test as soon as it reports connected. Give it
    // a moment so the refreshed report can show the real exit instead of a
    // stale "tap the IP card" warning.
    if (state.connState == VpnConnectionState.connected) {
      final ipDeadline = DateTime.now().add(const Duration(seconds: 22));
      while (state.lastIpTest == null && DateTime.now().isBefore(ipDeadline)) {
        await Future<void>.delayed(const Duration(milliseconds: 700));
      }
    }
    await runDiagnostics();
  }

  Future<void> runDiagnostics() async {
    if (state.diagnosticsRunning || state.diagnosticsHealing) return;
    state = state.copyWith(
      diagnosticsRunning: true,
      diagnosticsHealing: false,
    );
    try {
      final report =
          await _bridge.runDiagnostics().timeout(const Duration(seconds: 100));
      state = state.copyWith(
        diagnosticsRunning: false,
        diagnosticsReport: report,
      );
      unawaited(_refreshStatusSnapshot());
    } catch (e) {
      state = state.copyWith(
        diagnosticsRunning: false,
        errorMessageFA: 'عیب‌یابی ناموفق بود: ${_cleanError(e)}',
      );
    }
  }

  Future<void> autoHealDiagnostics() async {
    if (state.diagnosticsRunning || state.diagnosticsHealing) return;
    state = state.copyWith(
      diagnosticsHealing: true,
      diagnosticsRunning: false,
      errorMessageFA: null,
    );
    try {
      final report = await _bridge
          .autoHealDiagnostics(tunnelMode: state.tunnelMode)
          .timeout(const Duration(minutes: 2));
      state = state.copyWith(
        diagnosticsHealing: false,
        diagnosticsReport: report,
      );
      await _refreshStatusSnapshot();
      if (state.connState == VpnConnectionState.connected) {
        _ipAutoTested = true;
      }
    } catch (e) {
      state = state.copyWith(
        diagnosticsHealing: false,
        errorMessageFA: 'خودترمیمی ناموفق بود: ${_cleanError(e)}',
      );
    }
  }

  Future<void> _refreshStatusSnapshot() async {
    try {
      final snap = await _bridge.getStatus();
      _applySnapshot(snap);
    } catch (_) {}
  }

  Future<void> toggleLanSharing(
      {required bool enabled, required bool usePin}) async {
    if (state.lanBusy) return;
    state = state.copyWith(lanBusy: true, errorMessageFA: null);
    try {
      final info =
          await _bridge.toggleLanShare(enabled: enabled, usePin: usePin);
      state = state.copyWith(lanBusy: false, lanShare: info);
    } catch (e) {
      state = state.copyWith(
        lanBusy: false,
        errorMessageFA: 'تغییر اشتراک LAN ناموفق بود: ${_cleanError(e)}',
      );
    }
  }

  Future<void> checkForUpdates({bool silent = false}) async {
    if (state.updateChecking) return;
    state = silent
        ? state.copyWith(updateChecking: true)
        : state.copyWith(updateChecking: true, errorMessageFA: null);
    try {
      final info = await _bridge.checkUpdate();
      state = state.copyWith(
        updateChecking: false,
        availableUpdate: info,
      );
      try {
        final prefs = await SharedPreferences.getInstance();
        await prefs.setInt(
          'last_update_check_ms',
          DateTime.now().millisecondsSinceEpoch,
        );
      } catch (_) {}
    } catch (e) {
      state = silent
          ? state.copyWith(updateChecking: false)
          : state.copyWith(
              updateChecking: false,
              errorMessageFA: 'بررسی به‌روزرسانی ناموفق بود: ${_cleanError(e)}',
            );
    }
  }

  void dismissUpdate() {
    state = state.copyWith(availableUpdate: null);
  }

  String _cleanError(Object error) {
    final raw = error
        .toString()
        .replaceFirst('Exception: ', '')
        .replaceFirst('PlatformException(', '')
        .trim();
    // The engine already phrases user-facing failures in Persian. Preserve that
    // specific explanation instead of flattening it into a generic English
    // pattern match (for example "end-to-end health check").
    if (RegExp(r'[\u0600-\u06FF]').hasMatch(raw)) return raw;
    final lower = raw.toLowerCase();
    if (lower.contains('end-to-end health check') ||
        lower.contains('no healthy') ||
        lower.contains('no candidate')) {
      return 'هیچ سروری پاسخ سالم نداد؛ چند لحظه بعد دوباره تلاش کنید.';
    }
    if (lower.contains('clash delay probe') ||
        lower.contains('timeout') ||
        lower.contains('timed out')) {
      return 'زمان پاسخ‌گویی سرور به پایان رسید؛ دوباره تلاش کنید.';
    }
    if (lower.contains('permission') || lower.contains('denied')) {
      return 'دسترسی لازم داده نشد؛ تنظیمات برنامه را بررسی کنید.';
    }
    if (RegExp(r'[A-Za-z]').hasMatch(raw)) {
      return 'اتصال برقرار نشد؛ لطفاً دوباره تلاش کنید.';
    }
    return raw;
  }

  Future<void> downloadAndInstallUpdate() async {
    final upd = state.availableUpdate;
    if (upd == null || !upd.updateAvailable || state.updateDownloading) return;
    state = state.copyWith(updateDownloading: true, errorMessageFA: null);
    try {
      await _bridge.downloadAndInstallUpdate(upd);
      state = state.copyWith(updateDownloading: false);
    } catch (e) {
      state = state.copyWith(
        updateDownloading: false,
        errorMessageFA: 'خطا در دریافت به‌روزرسانی: ${_cleanError(e)}',
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
