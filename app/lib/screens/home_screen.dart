import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

import '../models/vpn_models.dart';
import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';
import '../widgets/clickable_ip_card.dart';
import '../widgets/dual_selector_row.dart';
import '../widgets/glowing_connect_orb.dart';
import '../widgets/telemetry_bar.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

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
          // 1. Glowing Circular Connect Button
          Center(
            child: GlowingConnectOrb(
              state: state.connState,
              connectedSeconds: state.connectedSeconds,
              onTap: ctrl.toggleConnection,
            ),
          ),

          if (state.errorMessageFA != null) ...[
            const SizedBox(height: 8),
            SpatialGlassCard(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              borderColorOverride: c.error.withValues(alpha: 0.6),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      state.errorMessageFA!,
                      textAlign: TextAlign.center,
                      style: TextStyle(
                          color: c.error,
                          fontSize: 12,
                          fontWeight: FontWeight.w600),
                    ),
                  ),
                  IconButton(
                    visualDensity: VisualDensity.compact,
                    iconSize: 18,
                    color: c.error,
                    onPressed: ctrl.clearError,
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
            ),
          ],

          if (state.availableUpdate != null) ...[
            const SizedBox(height: 10),
            _UpdateAnnouncement(
              info: state.availableUpdate!,
              downloading: state.updateDownloading,
              onInstall: ctrl.downloadAndInstallUpdate,
              onDismiss: ctrl.dismissUpdate,
            ),
          ],

          const SizedBox(height: 16),

          // 2. Clickable IP Display Rectangle with Tap-to-Test
          ClickableIpCard(
            connState: state.connState,
            isTesting: state.ipTesting,
            lastResult: state.lastIpTest,
            onTapTest: ctrl.testIpRectangle,
          ),

          const SizedBox(height: 12),

          // 3. Dual Side-by-Side Sliding Selectors (Country Auto/Manual + Core/Auto Mode)
          DualSelectorRow(
            selectedCountry: state.country,
            selectedMode: state.engineMode,
            onCountryChanged: ctrl.selectCountry,
            onModeChanged: ctrl.selectEngineMode,
          ),

          const SizedBox(height: 12),

          // 4. Ping + Upload + Download Telemetry Bar
          TelemetryBar(
            connState: state.connState,
            realDelayMs: state.realDelayMs,
            uploadBps: state.uploadBps,
            downloadBps: state.downloadBps,
          ),
        ],
      ),
    );
  }
}

class _UpdateAnnouncement extends StatelessWidget {
  final AppUpdateInfo info;
  final bool downloading;
  final VoidCallback onInstall;
  final VoidCallback onDismiss;

  const _UpdateAnnouncement({
    required this.info,
    required this.downloading,
    required this.onInstall,
    required this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final note = info.changelogFA.isNotEmpty
        ? info.changelogFA.first
        : 'نسخه جدید با بهبود پایداری و امنیت آماده نصب است.';
    return SpatialGlassCard(
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 12),
      borderColorOverride: c.brandPrimary.withValues(alpha: 0.65),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(Icons.system_update_alt_rounded,
                  color: c.brandPrimary, size: 21),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'به‌روزرسانی ${info.version} منتشر شد',
                  style: TextStyle(
                      color: c.textPrimary,
                      fontSize: 13,
                      fontWeight: FontWeight.w800),
                ),
              ),
              IconButton(
                visualDensity: VisualDensity.compact,
                iconSize: 18,
                color: c.textMuted,
                onPressed: downloading ? null : onDismiss,
                icon: const Icon(Icons.close_rounded),
              ),
            ],
          ),
          Padding(
            padding: const EdgeInsets.only(left: 29, right: 8),
            child: Text(
              note,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: c.textMuted, fontSize: 11, height: 1.5),
            ),
          ),
          const SizedBox(height: 9),
          Padding(
            padding: const EdgeInsets.only(left: 29, right: 8),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    'دانلود و نصب مستقیم داخل برنامه',
                    style: TextStyle(color: c.textMuted, fontSize: 10.5),
                  ),
                ),
                FilledButton.icon(
                  onPressed: downloading ? null : onInstall,
                  icon: downloading
                      ? const SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.download_rounded, size: 17),
                  label: Text(downloading ? 'در حال دانلود...' : 'دریافت و نصب'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
