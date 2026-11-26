import 'dart:io';
import '../models/vpn_models.dart';
import 'engine_bridge_android.dart';
import 'engine_bridge_windows.dart';

/// Platform-neutral contract between Flutter UI and the Go core engine.
abstract class EngineBridge {
  factory EngineBridge.create() {
    if (Platform.isWindows) {
      return WindowsEngineBridge();
    }
    return AndroidEngineBridge();
  }

  Future<Map<String, dynamic>> connect({
    required TunnelModeOption tunnelMode,
    required EngineModeOption engineMode,
    required CountryChoice country,
    required bool batterySaver,
  });

  Future<Map<String, dynamic>> disconnect();

  Future<Map<String, dynamic>> getStatus();

  Future<IpTestResultModel> testIpHealth();

  Future<DiagnosticsReportModel> runDiagnostics();

  Future<DiagnosticsReportModel> autoHealDiagnostics({
    required TunnelModeOption tunnelMode,
  });

  Future<LanShareModel> toggleLanShare(
      {required bool enabled, required bool usePin});

  Future<AppUpdateInfo?> checkUpdate();

  Future<void> downloadAndInstallUpdate(AppUpdateInfo info);

  Future<void> dispose();
}
