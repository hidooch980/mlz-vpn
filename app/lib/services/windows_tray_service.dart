import 'dart:io';
import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

/// Manages the Windows System Tray menu and graceful window close cleanup
/// so elevated TUN processes never stay orphaned when the user exits MLZ VPN.
class WindowsTrayService with TrayListener, WindowListener {
  final Future<void> Function() onToggleConnect;
  final Future<void> Function() onAppExitCleanup;

  WindowsTrayService({
    required this.onToggleConnect,
    required this.onAppExitCleanup,
  });

  Future<void> init() async {
    if (!Platform.isWindows) return;
    try {
      windowManager.addListener(this);
      await windowManager.setPreventClose(true);

      trayManager.addListener(this);
      await trayManager.setToolTip('MLZ VPN');
      await trayManager.setContextMenu(
        Menu(
          items: [
            MenuItem(key: 'show', label: 'باز کردن MLZ VPN'),
            MenuItem(key: 'toggle', label: 'اتصال / قطع سریع'),
            MenuItem.separator(),
            MenuItem(key: 'exit', label: 'خروج کامل'),
          ],
        ),
      );
    } catch (_) {
      // Headless tests or environments without tray icon assets skip silently.
    }
  }

  @override
  void onTrayIconMouseDown() {
    windowManager.show();
    windowManager.focus();
  }

  @override
  void onTrayMenuItemClick(MenuItem menuItem) {
    switch (menuItem.key) {
      case 'show':
        windowManager.show();
        windowManager.focus();
      case 'toggle':
        onToggleConnect();
      case 'exit':
        _shutdownAndExit();
    }
  }

  @override
  void onWindowClose() {
    // Hide to tray instead of abrupt termination; full exit is available in tray menu.
    windowManager.hide();
  }

  Future<void> _shutdownAndExit() async {
    try {
      await onAppExitCleanup();
    } finally {
      await trayManager.destroy();
      await windowManager.destroy();
      exit(0);
    }
  }
}
