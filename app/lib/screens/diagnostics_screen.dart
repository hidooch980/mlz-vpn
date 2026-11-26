import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';

import '../models/vpn_models.dart';
import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';

/// Full-screen smart diagnostics workspace. It runs the 9-step pipeline
/// automatically on open, shows every detail, and offers one-tap auto-healing.
class DiagnosticsScreen extends ConsumerStatefulWidget {
  const DiagnosticsScreen({super.key});

  @override
  ConsumerState<DiagnosticsScreen> createState() => _DiagnosticsScreenState();
}

class _DiagnosticsScreenState extends ConsumerState<DiagnosticsScreen> {
  static const _stages = <String>[
    'بررسی اتصال پایه و کارت شبکه',
    'هماهنگی ساعت و گواهی‌های TLS',
    'آزمون DNS و تشخیص مسمومیت',
    'بررسی تداخل پراکسی و پورت',
    'تشخیص محدودیت UDP و DPI',
    'آزمون دسترسی مستقیم به سایت‌های ایرانی',
    'بازبینی فید امضاشده و سرورها',
    'آزمون آی‌پی خروج و تأخیر واقعی',
    'بررسی آماده‌باش هسته‌ها و مسیر پشتیبان',
  ];

  Timer? _stageTimer;
  Timer? _watchTimer;
  int _stage = 0;
  bool _autoWatch = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(vpnControllerProvider.notifier).runDiagnostics();
    });
    _stageTimer = Timer.periodic(const Duration(milliseconds: 1400), (_) {
      if (!mounted) return;
      if (ref.read(vpnControllerProvider).diagnosticsRunning ||
          ref.read(vpnControllerProvider).diagnosticsHealing) {
        setState(() => _stage = (_stage + 1) % _stages.length);
      }
    });
  }

  @override
  void dispose() {
    _stageTimer?.cancel();
    _watchTimer?.cancel();
    super.dispose();
  }

  void _toggleAutoWatch(bool value) {
    setState(() => _autoWatch = value);
    _watchTimer?.cancel();
    if (!value) return;
    _watchTimer = Timer.periodic(const Duration(seconds: 60), (_) {
      if (!mounted) return;
      final ctrl = ref.read(vpnControllerProvider.notifier);
      final state = ref.read(vpnControllerProvider);
      if (!state.diagnosticsRunning && !state.diagnosticsHealing) {
        ctrl.runDiagnostics();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(vpnControllerProvider);
    final ctrl = ref.read(vpnControllerProvider.notifier);
    final c = Theme.of(context).extension<MlzColors>()!;
    final report = state.diagnosticsReport;
    final busy = state.diagnosticsRunning || state.diagnosticsHealing;

    return Scaffold(
      backgroundColor: c.canvas,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('عیب‌یاب هوشمند و خودترمیمی',
                style: TextStyle(
                    color: c.textPrimary,
                    fontSize: 16,
                    fontWeight: FontWeight.w800)),
            Text('تشخیص خودکار علت قطعی و ترمیم بی‌خطر',
                style: TextStyle(color: c.textMuted, fontSize: 11)),
          ],
        ),
      ),
      body: Directionality(
        textDirection: TextDirection.rtl,
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 640),
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
              children: [
                if (state.errorMessageFA != null && !busy) ...[
                  _ErrorBanner(
                      message: 'آخرین خطای اتصال: ${state.errorMessageFA!}',
                      c: c),
                  const SizedBox(height: 10),
                ],
                if (report == null && busy)
                  _ScanningPanel(stage: _stages[_stage], c: c)
                else if (report == null)
                  _EmptyPanel(c: c, ctrl: ctrl, error: state.errorMessageFA)
                else
                  _ReportBody(
                    report: report,
                    c: c,
                    busy: busy,
                    healing: state.diagnosticsHealing,
                    stage: _stages[_stage],
                    onRerun: busy ? null : ctrl.runDiagnostics,
                    onHeal: busy ? null : ctrl.autoHealDiagnostics,
                    autoWatch: _autoWatch,
                    onAutoWatch: _toggleAutoWatch,
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ReportBody extends StatelessWidget {
  final DiagnosticsReportModel report;
  final MlzColors c;
  final bool busy;
  final bool healing;
  final String stage;
  final VoidCallback? onRerun;
  final VoidCallback? onHeal;
  final bool autoWatch;
  final ValueChanged<bool> onAutoWatch;

  const _ReportBody({
    required this.report,
    required this.c,
    required this.busy,
    required this.healing,
    required this.stage,
    required this.onRerun,
    required this.onHeal,
    required this.autoWatch,
    required this.onAutoWatch,
  });

  @override
  Widget build(BuildContext context) {
    final scoreColor = _scoreColor(report.healthScore, c);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SpatialGlassCard(
          padding: const EdgeInsets.all(18),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  _ScoreRing(
                      score: report.healthScore,
                      label: report.healthLabelFA,
                      color: scoreColor,
                      c: c),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(report.headlineFA,
                            style: TextStyle(
                                color: c.textPrimary,
                                fontSize: 17,
                                height: 1.4,
                                fontWeight: FontWeight.w900)),
                        const SizedBox(height: 6),
                        Text(
                          report.summaryFA,
                          style: TextStyle(
                              color: c.textMuted, fontSize: 12, height: 1.7),
                        ),
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 6,
                          runSpacing: 6,
                          children: [
                            _Chip(
                              icon: Icons.verified_outlined,
                              label: 'سلامت ${report.healthScore} از ۱۰۰',
                              color: scoreColor,
                            ),
                            if (report.autoFixCount > 0)
                              _Chip(
                                icon: Icons.auto_fix_high_rounded,
                                label:
                                    '${report.autoFixCount} مورد خودکار اصلاح شد',
                                color: c.connectedGlow,
                              ),
                            if (report.needsAction)
                              _Chip(
                                icon: Icons.priority_high_rounded,
                                label: 'نیاز به اقدام',
                                color: c.warning,
                              ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              if (busy) ...[
                const SizedBox(height: 14),
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: LinearProgressIndicator(
                    minHeight: 5,
                    color: c.brandPrimary,
                    backgroundColor: c.borderHairline,
                  ),
                ),
                const SizedBox(height: 7),
                Text(
                  healing ? 'در حال ترمیم و آزمون دوباره...' : stage,
                  style: TextStyle(color: c.brandPrimary, fontSize: 11),
                ),
              ],
              const SizedBox(height: 14),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  FilledButton.icon(
                    onPressed: report.hasAutoFixable ? onHeal : null,
                    icon: const Icon(Icons.build_circle_outlined, size: 18),
                    label: Text(
                        healing ? 'در حال خودترمیمی...' : 'رفع خودکار مشکلات'),
                  ),
                  OutlinedButton.icon(
                    onPressed: onRerun,
                    icon: const Icon(Icons.refresh_rounded, size: 18),
                    label: const Text('اجرای دوبارهٔ ۹ بررسی'),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              Text(
                _formatTimestamp(report.timestamp),
                style: TextStyle(color: c.textMuted, fontSize: 11),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        Row(
          children: [
            Icon(Icons.fact_check_outlined, size: 17, color: c.textMuted),
            const SizedBox(width: 6),
            Text('نتیجهٔ هر بررسی',
                style: TextStyle(
                    color: c.textPrimary,
                    fontSize: 14,
                    fontWeight: FontWeight.w800)),
            const Spacer(),
            Text('${report.steps.length} بررسی',
                style: TextStyle(color: c.textMuted, fontSize: 11)),
          ],
        ),
        const SizedBox(height: 10),
        ...report.steps.map((step) => Padding(
              padding: const EdgeInsets.only(bottom: 9),
              child: _StepCard(step: step, c: c),
            )),
        const SizedBox(height: 4),
        SpatialGlassCard(
          padding: const EdgeInsets.all(13),
          child: Row(
            children: [
              Icon(Icons.privacy_tip_outlined,
                  size: 18, color: c.connectedGlow),
              const SizedBox(width: 9),
              Expanded(
                child: Text(
                  'همهٔ بررسی‌ها روی همین دستگاه انجام می‌شود و گزارش به هیچ سروری فرستاده نمی‌شود. '
                  'خودترمیمی هرگز پراکسی یا تنظیمات شبکهٔ سیستم را بدون اجازه تغییر نمی‌دهد.',
                  style:
                      TextStyle(color: c.textMuted, fontSize: 11, height: 1.7),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        SpatialGlassCard(
          padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 4),
          child: SwitchListTile.adaptive(
            contentPadding: EdgeInsets.zero,
            value: autoWatch,
            onChanged: onAutoWatch,
            title: Text('پایش خودکار هر ۶۰ ثانیه',
                style: TextStyle(color: c.textPrimary, fontSize: 13)),
            subtitle: Text(
              'تا وقتی این صفحه باز است، بررسی‌ها تکرار می‌شوند و مشکلات جدید گزارش می‌شوند.',
              style: TextStyle(color: c.textMuted, fontSize: 11),
            ),
          ),
        ),
      ],
    );
  }

  Color _scoreColor(int score, MlzColors c) {
    if (score >= 90) return c.connectedGlow;
    if (score >= 75) return c.brandPrimary;
    if (score >= 55) return c.warning;
    return c.error;
  }

  String _formatTimestamp(int timestamp) {
    if (timestamp <= 0) return 'زمان اجرا نامشخص';
    final d = DateTime.fromMillisecondsSinceEpoch(timestamp * 1000);
    String two(int v) => v.toString().padLeft(2, '0');
    return 'آخرین اجرا: ${d.year}/${two(d.month)}/${two(d.day)} • '
        '${two(d.hour)}:${two(d.minute)}';
  }
}

class _ScoreRing extends StatelessWidget {
  final int score;
  final String label;
  final Color color;
  final MlzColors c;

  const _ScoreRing(
      {required this.score,
      required this.label,
      required this.color,
      required this.c});

  @override
  Widget build(BuildContext context) {
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0, end: score / 100),
      duration: const Duration(milliseconds: 900),
      curve: Curves.easeOutCubic,
      builder: (_, value, __) => SizedBox(
        width: 96,
        height: 96,
        child: Stack(
          alignment: Alignment.center,
          children: [
            SizedBox(
              width: 90,
              height: 90,
              child: CircularProgressIndicator(
                value: value,
                strokeWidth: 8,
                strokeCap: StrokeCap.round,
                color: color,
                backgroundColor: c.borderHairline,
              ),
            ),
            Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('${score}',
                    style: TextStyle(
                        color: c.textPrimary,
                        fontSize: 25,
                        fontWeight: FontWeight.w900)),
                Text(label,
                    style: TextStyle(
                        color: color,
                        fontSize: 10,
                        fontWeight: FontWeight.w700)),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _StepCard extends StatelessWidget {
  final DiagnosticStepModel step;
  final MlzColors c;

  const _StepCard({required this.step, required this.c});

  @override
  Widget build(BuildContext context) {
    final visual = _visual();
    return SpatialGlassCard(
      padding: const EdgeInsets.all(13),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(
                  color: visual.$1.withValues(alpha: 0.14),
                  shape: BoxShape.circle,
                ),
                child: Icon(visual.$2, size: 17, color: visual.$1),
              ),
              const SizedBox(width: 9),
              Expanded(
                child: Text(step.titleFA,
                    style: TextStyle(
                        color: c.textPrimary,
                        fontSize: 13,
                        height: 1.5,
                        fontWeight: FontWeight.w700)),
              ),
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: visual.$1.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(visual.$3,
                    style: TextStyle(
                        color: visual.$1,
                        fontSize: 10,
                        fontWeight: FontWeight.w800)),
              ),
            ],
          ),
          const SizedBox(height: 9),
          Padding(
            padding: const EdgeInsets.only(right: 37),
            child: Text(step.detailFA,
                style:
                    TextStyle(color: c.textMuted, fontSize: 11, height: 1.75)),
          ),
          if (step.isFailed &&
              step.canAutoFix &&
              step.fixLabelFA.isNotEmpty) ...[
            const SizedBox(height: 9),
            Padding(
              padding: const EdgeInsets.only(right: 37),
              child: Row(
                children: [
                  Icon(Icons.auto_fix_high_rounded,
                      size: 14, color: c.brandPrimary),
                  const SizedBox(width: 5),
                  Expanded(
                    child: Text('قابل رفع خودکار: ${step.fixLabelFA}',
                        style: TextStyle(
                            color: c.brandPrimary,
                            fontSize: 10.5,
                            fontWeight: FontWeight.w600)),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  (Color, IconData, String) _visual() {
    switch (step.status) {
      case 'ok':
        return (c.connectedGlow, Icons.check_circle_outline_rounded, 'سالم');
      case 'fixed':
        return (c.brandPrimary, Icons.build_circle_outlined, 'اصلاح شد');
      case 'info':
        return (c.brandPrimary, Icons.info_outline_rounded, 'اطلاع');
      default:
        return (c.error, Icons.error_outline_rounded, 'نیاز به بررسی');
    }
  }
}

class _Chip extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;

  const _Chip({required this.icon, required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: color),
          const SizedBox(width: 4),
          Text(label,
              style: TextStyle(
                  color: color, fontSize: 10, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class _ScanningPanel extends StatelessWidget {
  final String stage;
  final MlzColors c;

  const _ScanningPanel({required this.stage, required this.c});

  @override
  Widget build(BuildContext context) {
    return SpatialGlassCard(
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          SizedBox(
            width: 62,
            height: 62,
            child: CircularProgressIndicator(
                strokeWidth: 6, color: c.brandPrimary),
          ),
          const SizedBox(height: 16),
          Text('در حال اجرای ۹ بررسی هوشمند',
              style: TextStyle(
                  color: c.textPrimary,
                  fontSize: 15,
                  fontWeight: FontWeight.w800)),
          const SizedBox(height: 8),
          Text(stage,
              textAlign: TextAlign.center,
              style: TextStyle(color: c.brandPrimary, fontSize: 12)),
          const SizedBox(height: 12),
          Text(
            'DNS، محدودیت UDP/DPI، ساعت، تداخل پراکسی، فید امضاشده و خروج تونل بررسی می‌شوند.',
            textAlign: TextAlign.center,
            style: TextStyle(color: c.textMuted, fontSize: 11, height: 1.7),
          ),
        ],
      ),
    );
  }
}

class _EmptyPanel extends StatelessWidget {
  final MlzColors c;
  final VpnController ctrl;
  final String? error;

  const _EmptyPanel({required this.c, required this.ctrl, this.error});

  @override
  Widget build(BuildContext context) {
    return SpatialGlassCard(
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          Icon(Icons.health_and_safety_outlined, size: 40, color: c.textMuted),
          const SizedBox(height: 10),
          Text(error ?? 'هنوز گزارشی در دسترس نیست.',
              textAlign: TextAlign.center,
              style: TextStyle(color: c.textMuted, fontSize: 12)),
          const SizedBox(height: 12),
          FilledButton.icon(
            onPressed: ctrl.runDiagnostics,
            icon: const Icon(Icons.refresh_rounded, size: 18),
            label: const Text('اجرای عیب‌یابی'),
          ),
        ],
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String message;
  final MlzColors c;

  const _ErrorBanner({required this.message, required this.c});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: c.error.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: c.error.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline_rounded, color: c.error, size: 18),
          const SizedBox(width: 8),
          Expanded(
              child: Text(message,
                  style: TextStyle(color: c.error, fontSize: 11.5))),
        ],
      ),
    );
  }
}
