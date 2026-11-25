import 'package:material_ui/material_ui.dart';
import '../models/vpn_models.dart';
import '../theme/mlz_theme.dart';

/// Bottom telemetry bar showing Real Delay ping, Upload speed/total, and Download speed/total.
class TelemetryBar extends StatelessWidget {
  final VpnConnectionState connState;
  final int realDelayMs;
  final int uploadBps;
  final int downloadBps;

  const TelemetryBar({
    super.key,
    required this.connState,
    required this.realDelayMs,
    required this.uploadBps,
    required this.downloadBps,
  });

  String _formatRate(int bps) {
    if (bps <= 0) return '0 KB/s';
    final kbps = bps / 1024.0;
    if (kbps < 1024) return '${kbps.toStringAsFixed(0)} KB/s';
    return '${(kbps / 1024.0).toStringAsFixed(1)} MB/s';
  }

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final isConnected = connState == VpnConnectionState.connected;

    Color pingColor = c.textMuted;
    if (isConnected && realDelayMs > 0) {
      pingColor = realDelayMs < 100 ? c.connectedGlow : (realDelayMs < 200 ? c.warning : c.error);
    }

    return SpatialGlassCard(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 14),
      child: Row(
        children: [
          _metric(c, 'پینگ واقعی', isConnected && realDelayMs > 0 ? '$realDelayMs ms' : '--', pingColor),
          _divider(c),
          _metric(c, 'ارسال ↑', isConnected ? _formatRate(uploadBps) : '0 KB/s', c.textPrimary),
          _divider(c),
          _metric(c, 'دریافت ↓', isConnected ? _formatRate(downloadBps) : '0 KB/s', c.textPrimary),
        ],
      ),
    );
  }

  Widget _divider(MlzColors c) {
    return Container(width: 1, height: 30, color: c.borderHairline);
  }

  Widget _metric(MlzColors c, String label, String value, Color valueColor) {
    return Expanded(
      child: Column(
        children: [
          Text(label, style: TextStyle(color: c.textMuted, fontSize: 12)),
          const SizedBox(height: 5),
          Directionality(
            textDirection: TextDirection.ltr,
            child: Text(
              value,
              style: TextStyle(
                color: valueColor,
                fontSize: 14,
                fontWeight: FontWeight.w800,
                fontFeatures: const [FontFeature.tabularFigures()],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
