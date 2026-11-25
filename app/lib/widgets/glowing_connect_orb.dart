import 'dart:math' as math;
import 'package:material_ui/material_ui.dart';
import '../models/vpn_models.dart';
import '../theme/mlz_theme.dart';

/// 2026/2027 glowing circular Connect button with multi-stage aura and spring pulse.
class GlowingConnectOrb extends StatefulWidget {
  final VpnConnectionState state;
  final int connectedSeconds;
  final VoidCallback onTap;

  const GlowingConnectOrb({
    super.key,
    required this.state,
    required this.connectedSeconds,
    required this.onTap,
  });

  @override
  State<GlowingConnectOrb> createState() => _GlowingConnectOrbState();
}

class _GlowingConnectOrbState extends State<GlowingConnectOrb> with TickerProviderStateMixin {
  late final AnimationController _pulseCtrl;
  late final AnimationController _spinCtrl;

  @override
  void initState() {
    super.initState();
    _pulseCtrl = AnimationController(vsync: this, duration: const Duration(milliseconds: 2400));
    _spinCtrl = AnimationController(vsync: this, duration: const Duration(milliseconds: 1800));
    _syncAnimations();
  }

  @override
  void didUpdateWidget(covariant GlowingConnectOrb oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.state != widget.state) {
      _syncAnimations();
    }
  }

  void _syncAnimations() {
    final isConnected = widget.state == VpnConnectionState.connected;
    final isConnecting = widget.state == VpnConnectionState.connecting || widget.state == VpnConnectionState.healing;

    if (isConnected) {
      if (!_pulseCtrl.isAnimating) _pulseCtrl.repeat(reverse: true);
    } else {
      _pulseCtrl.stop();
      _pulseCtrl.value = 0;
    }

    if (isConnecting) {
      if (!_spinCtrl.isAnimating) _spinCtrl.repeat();
    } else {
      _spinCtrl.stop();
      _spinCtrl.value = 0;
    }
  }

  @override
  void dispose() {
    _pulseCtrl.dispose();
    _spinCtrl.dispose();
    super.dispose();
  }

  String _formatDuration(int totalSeconds) {
    final h = (totalSeconds ~/ 3600).toString().padLeft(2, '0');
    final m = ((totalSeconds % 3600) ~/ 60).toString().padLeft(2, '0');
    final s = (totalSeconds % 60).toString().padLeft(2, '0');
    return '$h:$m:$s';
  }

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final isConnected = widget.state == VpnConnectionState.connected;
    final isConnecting = widget.state == VpnConnectionState.connecting || widget.state == VpnConnectionState.healing;
    final isError = widget.state == VpnConnectionState.error;

    final Color coreTop;
    final Color coreBottom;
    final Color glowColor;
    final String title;
    final String sub;

    if (isConnected) {
      coreTop = const Color(0xFF34D399);
      coreBottom = c.connectedGlow;
      glowColor = c.connectedGlow;
      title = 'متصل';
      sub = _formatDuration(widget.connectedSeconds);
    } else if (isConnecting) {
      coreTop = const Color(0xFFFBBF24);
      coreBottom = c.warning;
      glowColor = c.warning;
      title = widget.state == VpnConnectionState.healing ? 'خودترمیمی' : 'صبر کنید';
      sub = 'جستجوی بهترین مسیر...';
    } else if (isError) {
      coreTop = const Color(0xFFFB7185);
      coreBottom = c.error;
      glowColor = c.error;
      title = 'تلاش مجدد';
      sub = 'خطا در اتصال';
    } else {
      coreTop = c.brandPrimary;
      coreBottom = c.brandSecondary;
      glowColor = c.brandPrimary;
      title = 'اتصال';
      sub = 'لمس برای شروع';
    }

    return AnimatedBuilder(
      animation: Listenable.merge([_pulseCtrl, _spinCtrl]),
      builder: (context, _) {
        final breath = isConnected ? (1.0 + _pulseCtrl.value * 0.045) : 1.0;
        final glowBlur = isConnected ? (28.0 + _pulseCtrl.value * 36.0) : 18.0;
        final glowAlpha = isConnected ? (0.48 + _pulseCtrl.value * 0.28) : 0.22;

        return SizedBox(
          width: 210,
          height: 210,
          child: Stack(
            alignment: Alignment.center,
            children: [
              // Outer ambient aura
              Container(
                width: 184 * breath,
                height: 184 * breath,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  boxShadow: [
                    BoxShadow(
                      color: glowColor.withValues(alpha: glowAlpha),
                      blurRadius: glowBlur,
                      spreadRadius: isConnected ? 8 : 2,
                    ),
                    if (isConnected)
                      BoxShadow(
                        color: c.brandPrimary.withValues(alpha: 0.26),
                        blurRadius: glowBlur + 20,
                        spreadRadius: 4,
                      ),
                  ],
                ),
              ),

              // Rotating orbital ring while connecting / healing
              if (isConnecting)
                Transform.rotate(
                  angle: _spinCtrl.value * 2 * math.pi,
                  child: Container(
                    width: 182,
                    height: 182,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: glowColor.withValues(alpha: 0.75),
                        width: 2.5,
                      ),
                    ),
                    child: Align(
                      alignment: Alignment.topCenter,
                      child: Container(
                        width: 10,
                        height: 10,
                        decoration: BoxDecoration(color: glowColor, shape: BoxShape.circle),
                      ),
                    ),
                  ),
                ),

              // Main tactile glowing circle
              Transform.scale(
                scale: breath,
                child: Material(
                  color: Colors.transparent,
                  shape: const CircleBorder(),
                  child: InkWell(
                    onTap: widget.onTap,
                    customBorder: const CircleBorder(),
                    child: Ink(
                      width: 152,
                      height: 152,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        gradient: LinearGradient(
                          begin: Alignment.topRight,
                          end: Alignment.bottomLeft,
                          colors: [coreTop, coreBottom],
                        ),
                        border: Border.all(
                          color: Colors.white.withValues(alpha: isConnected ? 0.55 : 0.28),
                          width: 2,
                        ),
                      ),
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Text(
                            title,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 22,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                          const SizedBox(height: 6),
                          Directionality(
                            textDirection: isConnected ? TextDirection.ltr : TextDirection.rtl,
                            child: Text(
                              sub,
                              style: TextStyle(
                                color: Colors.white.withValues(alpha: 0.92),
                                fontSize: 12,
                                fontFeatures: const [FontFeature.tabularFigures()],
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
