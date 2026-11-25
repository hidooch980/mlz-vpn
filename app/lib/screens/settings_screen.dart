import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

import '../models/vpn_models.dart';
import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';
import '../widgets/settings_sheets.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(vpnControllerProvider);
    final ctrl = ref.read(vpnControllerProvider.notifier);
    final c = Theme.of(context).extension<MlzColors>()!;

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SpatialGlassCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('ظاهر و رفتار شبکه', style: TextStyle(color: c.textPrimary, fontSize: 15, fontWeight: FontWeight.w800)),
                const SizedBox(height: 14),
                _themePicker(c, state.themeMode, ctrl.setThemeMode),
                const SizedBox(height: 14),
                if (Platform.isWindows) ...[
                  Text('روش اتصال در ویندوز', style: TextStyle(color: c.textMuted, fontSize: 12)),
                  const SizedBox(height: 8),
                  _pillGroup<TunnelModeOption>(
                    c: c,
                    current: state.tunnelMode,
                    items: const [
                      (TunnelModeOption.tun, 'TUN (ادمین)'),
                      (TunnelModeOption.proxy, 'Proxy (بدون ادمین)'),
                    ],
                    onSelect: ctrl.setTunnelMode,
                  ),
                  const SizedBox(height: 14),
                ],
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        'مسیریابی سایت‌های ایرانی (.ir)',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: c.textPrimary, fontSize: 13),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'مستقیم و فعال ✓',
                      style: TextStyle(color: c.connectedGlow, fontSize: 13, fontWeight: FontWeight.w700),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  title: Text('حالت صرفه‌جویی باتری', style: TextStyle(color: c.textPrimary, fontSize: 13)),
                  subtitle: Text('تنظیم تطبیقی تست‌ها و همگام‌سازی ۱۵ دقیقه‌ای', style: TextStyle(color: c.textMuted, fontSize: 12)),
                  value: state.batterySaver,
                  onChanged: ctrl.setBatterySaver,
                ),
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  title: Text('قفل برنامه با اثرانگشت / PIN', style: TextStyle(color: c.textPrimary, fontSize: 13)),
                  value: state.appLockEnabled,
                  onChanged: ctrl.setAppLock,
                ),
              ],
            ),
          ),

          const SizedBox(height: 12),

          SpatialGlassCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('ابزارهای هوشمند و به‌روزرسانی', style: TextStyle(color: c.textPrimary, fontSize: 15, fontWeight: FontWeight.w800)),
                const SizedBox(height: 10),
                _actionTile(c, Icons.health_and_safety_outlined, 'عیب‌یابی هوشمند و خودترمیمی', () => openDiagnosticsSheet(context, state, ctrl)),
                const SizedBox(height: 8),
                _actionTile(c, Icons.wifi_tethering_rounded, 'اشتراک نقطه اتصال (LAN / Hotspot)', () => openLanShareSheet(context, state, ctrl)),
                const SizedBox(height: 8),
                _actionTile(c, Icons.system_update_alt_rounded, 'آپدیت از گیت‌هاب و Changelog', () => openChangelogSheet(context, state, ctrl)),
              ],
            ),
          ),

          const SizedBox(height: 14),
          Center(
            child: Text('MLZ VPN • متن‌باز تحت مجوز MIT', style: TextStyle(color: c.textMuted, fontSize: 12)),
          ),
        ],
      ),
    );
  }

  Widget _themePicker(MlzColors c, ThemeMode current, ValueChanged<ThemeMode> onChanged) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('تم برنامه', style: TextStyle(color: c.textMuted, fontSize: 12)),
        const SizedBox(height: 8),
        _pillGroup<ThemeMode>(
          c: c,
          current: current,
          items: const [
            (ThemeMode.system, 'سیستم'),
            (ThemeMode.dark, 'تاریک'),
            (ThemeMode.light, 'روشن'),
          ],
          onSelect: onChanged,
        ),
      ],
    );
  }

  Widget _pillGroup<T>({
    required MlzColors c,
    required T current,
    required List<(T, String)> items,
    required ValueChanged<T> onSelect,
  }) {
    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: c.elevatedSurface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: c.borderHairline),
      ),
      child: Row(
        children: items.map((item) {
          final selected = item.$1 == current;
          return Expanded(
            child: InkWell(
              onTap: () => onSelect(item.$1),
              borderRadius: BorderRadius.circular(10),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 6),
                decoration: BoxDecoration(
                  color: selected ? c.brandPrimary.withValues(alpha: 0.22) : Colors.transparent,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(
                    color: selected ? c.brandPrimary.withValues(alpha: 0.65) : Colors.transparent,
                  ),
                ),
                alignment: Alignment.center,
                child: Text(
                  item.$2,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: selected ? c.brandPrimary : c.textMuted,
                    fontSize: 12,
                    fontWeight: selected ? FontWeight.w800 : FontWeight.w600,
                  ),
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _actionTile(MlzColors c, IconData icon, String title, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
        decoration: BoxDecoration(
          color: c.elevatedSurface,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: c.borderHairline),
        ),
        child: Row(
          children: [
            Icon(icon, size: 20, color: c.brandPrimary),
            const SizedBox(width: 10),
            Expanded(child: Text(title, style: TextStyle(color: c.textPrimary, fontSize: 13, fontWeight: FontWeight.w600))),
            Icon(Icons.chevron_left_rounded, color: c.textMuted),
          ],
        ),
      ),
    );
  }
}
