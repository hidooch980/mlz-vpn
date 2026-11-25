import 'dart:ui';
import 'package:material_ui/material_ui.dart';

/// 2026/2027 Spatial Glass & Material 3 Expressive visual tokens for MLZ VPN.
class MlzColors extends ThemeExtension<MlzColors> {
  final Color canvas;
  final Color glassCard;
  final Color elevatedSurface;
  final Color borderHairline;
  final Color specularHighlight;
  final Color textPrimary;
  final Color textMuted;
  final Color brandPrimary;
  final Color brandSecondary;
  final Color connectedGlow;
  final Color warning;
  final Color error;

  const MlzColors({
    required this.canvas,
    required this.glassCard,
    required this.elevatedSurface,
    required this.borderHairline,
    required this.specularHighlight,
    required this.textPrimary,
    required this.textMuted,
    required this.brandPrimary,
    required this.brandSecondary,
    required this.connectedGlow,
    required this.warning,
    required this.error,
  });

  static const dark = MlzColors(
    canvas: Color(0xFF080C16),
    glassCard: Color(0xCC11182A),
    elevatedSurface: Color(0xFF18223A),
    borderHairline: Color(0x2694A3B8),
    specularHighlight: Color(0x1FFFFFFF),
    textPrimary: Color(0xFFF8FAFC),
    textMuted: Color(0xFF94A3B8),
    brandPrimary: Color(0xFF38BDF8),
    brandSecondary: Color(0xFF3B82F6),
    connectedGlow: Color(0xFF10B981),
    warning: Color(0xFFF59E0B),
    error: Color(0xFFFB7185),
  );

  static const light = MlzColors(
    canvas: Color(0xFFF4F7FC),
    glassCard: Color(0xE6FFFFFF),
    elevatedSurface: Color(0xFFE9EFF9),
    borderHairline: Color(0x1A0F172A),
    specularHighlight: Color(0xCCFFFFFF),
    textPrimary: Color(0xFF0F172A),
    textMuted: Color(0xFF475569),
    brandPrimary: Color(0xFF0284C7),
    brandSecondary: Color(0xFF1D4ED8),
    connectedGlow: Color(0xFF059669),
    warning: Color(0xFFD97706),
    error: Color(0xFFE11D48),
  );

  @override
  MlzColors copyWith({
    Color? canvas,
    Color? glassCard,
    Color? elevatedSurface,
    Color? borderHairline,
    Color? specularHighlight,
    Color? textPrimary,
    Color? textMuted,
    Color? brandPrimary,
    Color? brandSecondary,
    Color? connectedGlow,
    Color? warning,
    Color? error,
  }) {
    return MlzColors(
      canvas: canvas ?? this.canvas,
      glassCard: glassCard ?? this.glassCard,
      elevatedSurface: elevatedSurface ?? this.elevatedSurface,
      borderHairline: borderHairline ?? this.borderHairline,
      specularHighlight: specularHighlight ?? this.specularHighlight,
      textPrimary: textPrimary ?? this.textPrimary,
      textMuted: textMuted ?? this.textMuted,
      brandPrimary: brandPrimary ?? this.brandPrimary,
      brandSecondary: brandSecondary ?? this.brandSecondary,
      connectedGlow: connectedGlow ?? this.connectedGlow,
      warning: warning ?? this.warning,
      error: error ?? this.error,
    );
  }

  @override
  MlzColors lerp(ThemeExtension<MlzColors>? other, double t) {
    if (other is! MlzColors) return this;
    return MlzColors(
      canvas: Color.lerp(canvas, other.canvas, t)!,
      glassCard: Color.lerp(glassCard, other.glassCard, t)!,
      elevatedSurface: Color.lerp(elevatedSurface, other.elevatedSurface, t)!,
      borderHairline: Color.lerp(borderHairline, other.borderHairline, t)!,
      specularHighlight: Color.lerp(specularHighlight, other.specularHighlight, t)!,
      textPrimary: Color.lerp(textPrimary, other.textPrimary, t)!,
      textMuted: Color.lerp(textMuted, other.textMuted, t)!,
      brandPrimary: Color.lerp(brandPrimary, other.brandPrimary, t)!,
      brandSecondary: Color.lerp(brandSecondary, other.brandSecondary, t)!,
      connectedGlow: Color.lerp(connectedGlow, other.connectedGlow, t)!,
      warning: Color.lerp(warning, other.warning, t)!,
      error: Color.lerp(error, other.error, t)!,
    );
  }
}

class MlzTheme {
  static ThemeData dark() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: MlzColors.dark.canvas,
      fontFamily: 'Vazirmatn',
      colorScheme: const ColorScheme.dark(
        primary: Color(0xFF38BDF8),
        secondary: Color(0xFF10B981),
        surface: Color(0xFF11182A),
      ),
      extensions: const [MlzColors.dark],
    );
  }

  static ThemeData light() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      scaffoldBackgroundColor: MlzColors.light.canvas,
      fontFamily: 'Vazirmatn',
      colorScheme: const ColorScheme.light(
        primary: Color(0xFF0284C7),
        secondary: Color(0xFF059669),
        surface: Color(0xFFFFFFFF),
      ),
      extensions: const [MlzColors.light],
    );
  }
}

/// Reusable 2026 spatial glass container with hairline border and subtle depth.
class SpatialGlassCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final double borderRadius;
  final VoidCallback? onTap;
  final Color? borderColorOverride;

  const SpatialGlassCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.borderRadius = 22,
    this.onTap,
    this.borderColorOverride,
  });

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    final radius = BorderRadius.circular(borderRadius);

    final box = ClipRRect(
      borderRadius: radius,
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 220),
          decoration: BoxDecoration(
            color: c.glassCard,
            borderRadius: radius,
            border: Border.all(
              color: borderColorOverride ?? c.borderHairline,
              width: 1.1,
            ),
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              colors: [c.specularHighlight.withValues(alpha: 0.06), Colors.transparent],
            ),
          ),
          child: Material(
            type: MaterialType.transparency,
            child: onTap == null
                ? Padding(padding: padding, child: child)
                : InkWell(
                    onTap: onTap,
                    borderRadius: radius,
                    child: Padding(padding: padding, child: child),
                  ),
          ),
        ),
      ),
    );

    return box;
  }
}
