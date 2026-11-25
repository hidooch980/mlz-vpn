import 'dart:io';

import 'package:material_ui/material_ui.dart';
import '../models/vpn_models.dart';
import '../theme/mlz_theme.dart';

/// Two aligned selector cards opening sliding drawers:
/// 1) Country selector (Auto / Manual)
/// 2) Connection mode selector (Auto / Core-based strategies)
class DualSelectorRow extends StatelessWidget {
  final CountryChoice selectedCountry;
  final EngineModeOption selectedMode;
  final ValueChanged<CountryChoice> onCountryChanged;
  final ValueChanged<EngineModeOption> onModeChanged;

  const DualSelectorRow({
    super.key,
    required this.selectedCountry,
    required this.selectedMode,
    required this.onCountryChanged,
    required this.onModeChanged,
  });

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;

    return Row(
      children: [
        Expanded(
          child: SpatialGlassCard(
            onTap: () => _openCountryDrawer(context),
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        'کشور اتصال',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: c.textMuted, fontSize: 12),
                      ),
                    ),
                    Icon(Icons.keyboard_arrow_down_rounded, size: 18, color: c.textMuted),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  '${selectedCountry.flag} ${selectedCountry.nameFA}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: c.textPrimary, fontSize: 13, fontWeight: FontWeight.w700),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: SpatialGlassCard(
            onTap: () => _openEngineDrawer(context),
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        'حالت / هسته',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: c.textMuted, fontSize: 12),
                      ),
                    ),
                    Icon(Icons.keyboard_arrow_down_rounded, size: 18, color: c.textMuted),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  selectedMode.titleFA,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: c.textPrimary, fontSize: 13, fontWeight: FontWeight.w700),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  void _openCountryDrawer(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: c.canvas,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
      builder: (ctx) {
        return Directionality(
          textDirection: TextDirection.rtl,
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(width: 42, height: 4, decoration: BoxDecoration(color: c.textMuted.withValues(alpha: 0.4), borderRadius: BorderRadius.circular(8))),
                  const SizedBox(height: 14),
                  Align(
                    alignment: Alignment.centerRight,
                    child: Text('انتخاب کشور (اتوماتیک یا دستی)', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                  ),
                  const SizedBox(height: 12),
                  Flexible(
                    child: ListView.separated(
                      shrinkWrap: true,
                      itemCount: CountryChoice.presets.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 8),
                      itemBuilder: (_, idx) {
                        final item = CountryChoice.presets[idx];
                        final active = item.code == selectedCountry.code;
                        return SpatialGlassCard(
                          onTap: () {
                            Navigator.of(ctx).pop();
                            onCountryChanged(item);
                          },
                          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                          borderColorOverride: active ? c.brandPrimary : null,
                          child: Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Text('${item.flag}  ${item.nameFA}', style: TextStyle(color: c.textPrimary, fontWeight: active ? FontWeight.w800 : FontWeight.w600)),
                              Directionality(
                                textDirection: TextDirection.ltr,
                                child: Text(item.hintPing, style: TextStyle(color: active ? c.brandPrimary : c.textMuted, fontSize: 12, fontWeight: FontWeight.w700)),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  void _openEngineDrawer(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final modes = EngineModeOption.values
        .where((mode) => !Platform.isAndroid || mode.supportedOnAndroid)
        .toList(growable: false);
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: c.canvas,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
      builder: (ctx) {
        return Directionality(
          textDirection: TextDirection.rtl,
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(width: 42, height: 4, decoration: BoxDecoration(color: c.textMuted.withValues(alpha: 0.4), borderRadius: BorderRadius.circular(8))),
                  const SizedBox(height: 14),
                  Align(
                    alignment: Alignment.centerRight,
                    child: Text('حالت اتصال (اتوماتیک یا بر اساس هسته‌ها)', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                  ),
                  const SizedBox(height: 12),
                  Flexible(
                    child: ListView.separated(
                      shrinkWrap: true,
                      itemCount: modes.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 8),
                      itemBuilder: (_, idx) {
                        final mode = modes[idx];
                        final active = mode == selectedMode;
                        return SpatialGlassCard(
                          onTap: () {
                            Navigator.of(ctx).pop();
                            onModeChanged(mode);
                          },
                          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                          borderColorOverride: active ? c.brandPrimary : null,
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(mode.titleFA, style: TextStyle(color: active ? c.brandPrimary : c.textPrimary, fontSize: 14, fontWeight: FontWeight.w800)),
                              const SizedBox(height: 4),
                              Text(mode.subtitleFA, style: TextStyle(color: c.textMuted, fontSize: 12)),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
