import 'dart:io';
import 'package:cupertino_ui/cupertino_ui.dart';
import 'package:flutter_localizations/flutter_localizations.dart' show GlobalWidgetsLocalizations;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';
import 'package:window_manager/window_manager.dart';

import 'screens/main_shell.dart';
import 'services/windows_tray_service.dart';
import 'state/vpn_controller.dart';
import 'theme/mlz_theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final container = ProviderContainer();

  if (Platform.isWindows) {
    await windowManager.ensureInitialized();
    const opts = WindowOptions(
      size: Size(920, 640),
      minimumSize: Size(760, 560),
      center: true,
      title: 'MLZ VPN',
    );
    await windowManager.waitUntilReadyToShow(opts, () async {
      await windowManager.show();
      await windowManager.focus();
    });

    final tray = WindowsTrayService(
      onToggleConnect: () => container.read(vpnControllerProvider.notifier).toggleConnection(),
      onAppExitCleanup: () async {
        container.dispose();
      },
    );
    await tray.init();
  }

  runApp(
    UncontrolledProviderScope(
      container: container,
      child: const MlzVpnApp(),
    ),
  );
}

class MlzVpnApp extends ConsumerWidget {
  const MlzVpnApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeMode = ref.watch(vpnControllerProvider.select((s) => s.themeMode));

    return MaterialApp(
      title: 'MLZ VPN',
      debugShowCheckedModeBanner: false,
      locale: const Locale('fa', 'IR'),
      supportedLocales: const [Locale('fa', 'IR'), Locale('en', 'US')],
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      themeMode: themeMode,
      theme: MlzTheme.light(),
      darkTheme: MlzTheme.dark(),
      builder: (context, child) {
        return Directionality(
          textDirection: TextDirection.rtl,
          child: child ?? const SizedBox.shrink(),
        );
      },
      home: const MainShell(),
    );
  }
}
