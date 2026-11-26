import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:mlz_vpn/models/vpn_models.dart';
import 'package:mlz_vpn/screens/diagnostics_screen.dart';
import 'package:mlz_vpn/services/engine_bridge.dart';
import 'package:mlz_vpn/state/vpn_controller.dart';
import 'package:mlz_vpn/theme/mlz_theme.dart';
import 'package:shared_preferences/shared_preferences.dart';

DiagnosticsReportModel _report({bool healed = false}) {
  return DiagnosticsReportModel.fromJson({
    'timestamp': 1795673000,
    'summary_fa':
        healed ? 'همهٔ بررسی‌ها سالم‌اند.' : 'یک مورد نیاز به خودترمیمی دارد.',
    'headline_fa': healed ? 'همه‌چیز سالم است' : 'خروج تونل هنوز سالم نیست',
    'health_score': healed ? 100 : 86,
    'health_label_fa': healed ? 'عالی' : 'خوب',
    'auto_healed': healed,
    'auto_fix_count': healed ? 1 : 0,
    'needs_action': !healed,
    'steps': [
      for (final id in [
        'interface',
        'clock',
        'dns',
        'conflict',
        'dpi',
        'iran_direct',
        'feed',
        'exit',
        'cores'
      ])
        {
          'id': id,
          'title_fa':
              id == 'exit' ? 'سلامت خروج تونل و آی‌پی نهایی' : 'بررسی $id',
          'status': id == 'exit' && !healed
              ? 'failed'
              : (id == 'exit' ? 'fixed' : 'ok'),
          'severity': id == 'exit' && !healed ? 'warning' : 'ok',
          'detail_fa': id == 'exit'
              ? (healed
                  ? 'خروج تونل پس از خودترمیمی با موفقیت آزمون شد.'
                  : 'اتصال برقرار است ولی آزمون خروج هنوز نتیجهٔ سالمی ندارد.')
              : 'این بخش سالم است.',
          'auto_applied': id == 'exit' && healed,
          'can_auto_fix': id == 'exit',
          'fix_label_fa':
              id == 'exit' ? 'بازسازی اتصال و آزمون دوبارهٔ خروج' : '',
        }
    ],
  });
}

DiagnosticsReportModel _failedConnectionReport() {
  return DiagnosticsReportModel.fromJson({
    'timestamp': 1795673000,
    'summary_fa':
        'آخرین تلاش اتصال ناموفق بود: هیچ‌کدام از ۸ مسیر برتر پاسخ ندادند.',
    'headline_fa': 'اتصال VPN برقرار نشد',
    'health_score': 56,
    'health_label_fa': 'نیازمند توجه',
    'auto_healed': false,
    'auto_fix_count': 0,
    'needs_action': true,
    'steps': [
      for (final id in [
        'interface',
        'clock',
        'dns',
        'conflict',
        'dpi',
        'iran_direct',
        'feed',
        'exit',
        'cores'
      ])
        {
          'id': id,
          'title_fa': id == 'exit'
              ? 'سلامت خروج تونل و آی‌پی نهایی'
              : (id == 'cores'
                  ? 'آماده‌باش هسته‌ها و مسیرهای پشتیبان'
                  : 'بررسی $id'),
          'status': (id == 'exit' || id == 'cores') ? 'failed' : 'ok',
          'severity':
              id == 'cores' ? 'critical' : (id == 'exit' ? 'warning' : 'ok'),
          'detail_fa': id == 'cores' || id == 'exit'
              ? 'اتصال VPN برقرار نشد: هیچ‌کدام از ۸ مسیر برتر پاسخ ندادند.'
              : 'این بخش سالم است.',
          'auto_applied': false,
          'can_auto_fix': false,
          'fix_label_fa': '',
        }
    ],
  });
}

class _FakeBridge implements EngineBridge {
  _FakeBridge({this.connected = false});

  bool connected;
  int diagnosticsCalls = 0;
  int healCalls = 0;

  @override
  Future<DiagnosticsReportModel> runDiagnostics() async {
    diagnosticsCalls++;
    return _report();
  }

  @override
  Future<DiagnosticsReportModel> autoHealDiagnostics({
    required TunnelModeOption tunnelMode,
  }) async {
    healCalls++;
    return _report(healed: true);
  }

  @override
  Future<Map<String, dynamic>> getStatus() async =>
      {'state': connected ? 'connected' : 'disconnected'};

  @override
  Future<Map<String, dynamic>> connect({
    required TunnelModeOption tunnelMode,
    required EngineModeOption engineMode,
    required CountryChoice country,
    required bool batterySaver,
  }) async =>
      {'state': 'disconnected'};

  @override
  Future<Map<String, dynamic>> disconnect() async => {'state': 'disconnected'};

  @override
  Future<IpTestResultModel> testIpHealth() async =>
      IpTestResultModel.fromJson({'public_ip': '1.1.1.1'});

  @override
  Future<LanShareModel> toggleLanShare(
          {required bool enabled, required bool usePin}) async =>
      LanShareModel.fromJson({});

  @override
  Future<AppUpdateInfo?> checkUpdate() async => null;

  @override
  Future<void> downloadAndInstallUpdate(AppUpdateInfo info) async {}

  @override
  Future<void> dispose() async {}
}

class _FailedConnectionBridge extends _FakeBridge {
  @override
  Future<DiagnosticsReportModel> runDiagnostics() async {
    diagnosticsCalls++;
    return _failedConnectionReport();
  }
}

void main() {
  testWidgets('diagnostics runs automatically, shows details and heals',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    tester.view.physicalSize = const Size(920, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final bridge = _FakeBridge(connected: true);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          vpnControllerProvider.overrideWith((ref) => VpnController(bridge)),
        ],
        child: MaterialApp(
          theme: MlzTheme.dark(),
          home: const DiagnosticsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(bridge.diagnosticsCalls, 1);
    expect(find.text('عیب‌یاب هوشمند و خودترمیمی'), findsOneWidget);
    expect(find.text('سلامت 86 از ۱۰۰'), findsOneWidget);
    expect(find.text('نیاز به اقدام'), findsOneWidget);
    expect(find.text('سلامت خروج تونل و آی‌پی نهایی'), findsOneWidget);

    final heal = find.text('رفع خودکار مشکلات');
    await tester.ensureVisible(heal);
    await tester.tap(heal);
    await tester.pumpAndSettle();

    expect(bridge.healCalls, 1);
    expect(find.text('همه‌چیز سالم است'), findsOneWidget);
    expect(find.text('خروج تونل پس از خودترمیمی با موفقیت آزمون شد.'),
        findsOneWidget);
  });

  testWidgets(
      'failed connection offers a real reconnect, not just connect-first',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    tester.view.physicalSize = const Size(920, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          vpnControllerProvider
              .overrideWith((ref) => VpnController(_FailedConnectionBridge())),
        ],
        child: MaterialApp(
          theme: MlzTheme.dark(),
          home: const DiagnosticsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('اتصال VPN برقرار نشد'), findsOneWidget);
    expect(find.textContaining('هیچ‌کدام از ۸ مسیر'), findsWidgets);

    final retry = find.text('تلاش دوباره برای اتصال');
    expect(retry, findsOneWidget);
    await tester.ensureVisible(retry);
    final button = tester.widget<FilledButton>(
      find.ancestor(of: retry, matching: find.byType(FilledButton)),
    );
    expect(button.onPressed, isNotNull);
    expect(find.text('رفع خودکار مشکلات'), findsNothing);
  });

  testWidgets('diagnostics screen visual snapshot', (tester) async {
    SharedPreferences.setMockInitialValues({});
    tester.view.physicalSize = const Size(920, 1100);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final bridge = _FakeBridge();
    final key = GlobalKey();
    await tester.pumpWidget(
      RepaintBoundary(
        key: key,
        child: ProviderScope(
          overrides: [
            vpnControllerProvider.overrideWith((ref) => VpnController(bridge)),
          ],
          child: MaterialApp(
            theme: MlzTheme.dark(),
            home: const DiagnosticsScreen(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.runAsync(() async {
      final boundary =
          key.currentContext!.findRenderObject()! as RenderRepaintBoundary;
      final image = await boundary.toImage(pixelRatio: 1);
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      final path =
          Platform.environment['MLZ_CAPTURE_PATH'] ?? 'diagnostics-ui.png';
      await File(path).writeAsBytes(bytes!.buffer.asUint8List());
    });
  }, skip: Platform.environment['MLZ_CAPTURE_UI'] != '1');
}
