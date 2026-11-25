import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

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
              child: Text(
                state.errorMessageFA!,
                textAlign: TextAlign.center,
                style: TextStyle(color: c.error, fontSize: 12, fontWeight: FontWeight.w600),
              ),
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
