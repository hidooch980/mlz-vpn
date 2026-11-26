import 'package:material_ui/material_ui.dart';
import '../models/vpn_models.dart';
import '../theme/mlz_theme.dart';

/// IP display rectangle supporting tap-to-test for live IP, real delay, DNS leak, and .ir bypass checks.
class ClickableIpCard extends StatelessWidget {
  final VpnConnectionState connState;
  final bool isTesting;
  final IpTestResultModel? lastResult;
  final VoidCallback onTapTest;

  const ClickableIpCard({
    super.key,
    required this.connState,
    required this.isTesting,
    required this.lastResult,
    required this.onTapTest,
  });

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final isConnected = connState == VpnConnectionState.connected;

    final ipLine = lastResult != null
        ? '${lastResult!.publicIp}  •  ${lastResult!.countryNameFA}'
        : (isConnected
            ? 'برای دریافت آی‌پی خروجی، این کارت را لمس کنید'
            : 'برای بررسی آی‌پی واقعی کلیک کنید');

    return SpatialGlassCard(
      onTap: onTapTest,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      borderColorOverride: isConnected ? c.connectedGlow.withValues(alpha: 0.35) : null,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  'آی‌پی فعلی شما (لمس برای تست)',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: c.textMuted, fontSize: 12),
                ),
              ),
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                  color: c.elevatedSurface,
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (isTesting)
                      SizedBox(
                        width: 12,
                        height: 12,
                        child: CircularProgressIndicator(strokeWidth: 2, color: c.brandPrimary),
                      )
                    else
                      Icon(Icons.verified_user_outlined, size: 14, color: isConnected ? c.connectedGlow : c.brandPrimary),
                    const SizedBox(width: 6),
                    Text(
                      isTesting ? 'در حال تست...' : 'تست آی‌پی',
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: isConnected ? c.connectedGlow : c.brandPrimary,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Directionality(
            textDirection: TextDirection.ltr,
            child: Align(
              alignment: Alignment.centerRight,
              child: Text(
                ipLine,
                style: TextStyle(
                  color: c.textPrimary,
                  fontSize: 15,
                  fontWeight: FontWeight.w700,
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
            ),
          ),
          if (lastResult != null) ...[
            const SizedBox(height: 6),
            Text(
              '${lastResult!.dnsProtected ? 'DNS امن ✓' : 'بدون تونل'}  •  ${lastResult!.iranDirectOk ? 'سایت‌های .ir مستقیم ✓' : 'بررسی مسیر داخلی'}  •  ${lastResult!.realDelayMs} ms',
              style: TextStyle(
                color: lastResult!.dnsProtected ? c.connectedGlow : c.warning,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ],
      ),
    );
  }
}
