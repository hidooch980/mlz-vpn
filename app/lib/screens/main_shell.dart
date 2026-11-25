import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';
import '../widgets/biometric_lock_gate.dart';
import 'home_screen.dart';
import 'settings_screen.dart';

/// Root navigation shell strictly containing ONLY 2 items: Home (خانه) and Settings (تنظیمات).
class MainShell extends ConsumerStatefulWidget {
  const MainShell({super.key});

  @override
  ConsumerState<MainShell> createState() => _MainShellState();
}

class _MainShellState extends ConsumerState<MainShell> {
  int _tabIndex = 0; // 0 = Home, 1 = Settings

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(vpnControllerProvider);
    final ctrl = ref.read(vpnControllerProvider.notifier);
    final c = Theme.of(context).extension<MlzColors>()!;

    if (state.appLockEnabled && !state.isUnlocked) {
      return BiometricLockGate(onUnlocked: ctrl.markUnlocked);
    }

    final isDesktop = MediaQuery.of(context).size.width >= 760;
    final body = IndexedStack(
      index: _tabIndex,
      children: const [HomeScreen(), SettingsScreen()],
    );

    if (isDesktop) {
      return Scaffold(
        backgroundColor: c.canvas,
        body: Row(
          children: [
            // Strictly 2-item RTL glass sidebar on Windows
            Container(
              width: 215,
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: c.glassCard,
                border: Border(left: BorderSide(color: c.borderHairline)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 32,
                        height: 32,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(10),
                          gradient: LinearGradient(colors: [c.brandPrimary, c.brandSecondary]),
                        ),
                        alignment: Alignment.center,
                        child: const Text('M', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w900)),
                      ),
                      const SizedBox(width: 10),
                      Text('MLZ VPN', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                    ],
                  ),
                  const SizedBox(height: 24),
                  _navButton(c, 0, Icons.shield_rounded, 'خانه'),
                  const SizedBox(height: 8),
                  _navButton(c, 1, Icons.tune_rounded, 'تنظیمات'),
                  const Spacer(),
                  SpatialGlassCard(
                    padding: const EdgeInsets.all(12),
                    child: Text(
                      state.tunnelMode.labelFA,
                      style: TextStyle(color: c.textMuted, fontSize: 11, height: 1.5),
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 560),
                  child: body,
                ),
              ),
            ),
          ],
        ),
      );
    }

    return Scaffold(
      backgroundColor: c.canvas,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        title: Row(
          children: [
            Container(
              width: 28,
              height: 28,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(9),
                gradient: LinearGradient(colors: [c.brandPrimary, c.brandSecondary]),
              ),
              alignment: Alignment.center,
              child: const Text('M', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w900)),
            ),
            const SizedBox(width: 10),
            Text('MLZ VPN', style: TextStyle(color: c.textPrimary, fontSize: 17, fontWeight: FontWeight.w800)),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'تغییر تم',
            onPressed: () {
              final next = Theme.of(context).brightness == Brightness.dark ? ThemeMode.light : ThemeMode.dark;
              ctrl.setThemeMode(next);
            },
            icon: Icon(
              Theme.of(context).brightness == Brightness.dark ? Icons.light_mode_rounded : Icons.dark_mode_rounded,
              color: c.textPrimary,
            ),
          ),
        ],
      ),
      body: body,
      bottomNavigationBar: Padding(
        padding: const EdgeInsets.fromLTRB(20, 4, 20, 14),
        child: SpatialGlassCard(
          borderRadius: 24,
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          child: Row(
            children: [
              Expanded(child: _navButton(c, 0, Icons.shield_rounded, 'خانه')),
              const SizedBox(width: 8),
              Expanded(child: _navButton(c, 1, Icons.tune_rounded, 'تنظیمات')),
            ],
          ),
        ),
      ),
    );
  }

  Widget _navButton(MlzColors c, int index, IconData icon, String label) {
    final active = _tabIndex == index;
    return InkWell(
      onTap: () => setState(() => _tabIndex = index),
      borderRadius: BorderRadius.circular(16),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
        decoration: BoxDecoration(
          color: active ? c.brandPrimary.withValues(alpha: 0.16) : Colors.transparent,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 19, color: active ? c.brandPrimary : c.textMuted),
            const SizedBox(width: 8),
            Flexible(
              child: Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  color: active ? c.brandPrimary : c.textMuted,
                  fontSize: 13,
                  fontWeight: active ? FontWeight.w800 : FontWeight.w600,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
