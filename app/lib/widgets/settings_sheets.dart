import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';

void openDiagnosticsSheet(BuildContext context, MlzAppState _, VpnController ctrl) {
  final c = Theme.of(context).extension<MlzColors>()!;
  ctrl.runDiagnostics();

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: c.canvas,
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
    builder: (ctx) {
      return Consumer(
        builder: (ctx, ref, _) {
          final liveState = ref.watch(vpnControllerProvider);
          return Directionality(
            textDirection: TextDirection.rtl,
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 14, 16, 24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text('عیب‌یاب و خودترمیمی هوشمند', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                    const SizedBox(height: 10),
                    if (liveState.diagnosticsRunning || liveState.diagnosticsReport == null)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 28),
                        child: Center(child: CircularProgressIndicator(color: c.brandPrimary)),
                      )
                    else ...[
                      SpatialGlassCard(
                        padding: const EdgeInsets.all(12),
                        child: Text(liveState.diagnosticsReport!.summaryFA, style: TextStyle(color: c.textPrimary, fontSize: 13)),
                      ),
                      const SizedBox(height: 10),
                      ...liveState.diagnosticsReport!.steps.map(
                        (step) => Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: SpatialGlassCard(
                            padding: const EdgeInsets.all(12),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Expanded(child: Text(step.titleFA, style: TextStyle(color: c.textPrimary, fontSize: 13))),
                                Text(
                                  step.status == 'ok' ? 'سالم ✓' : (step.status == 'fixed' ? 'اصلاح شد' : 'نیاز به بررسی'),
                                  style: TextStyle(
                                    fontSize: 12,
                                    fontWeight: FontWeight.w700,
                                    color: step.status == 'ok' ? c.connectedGlow : (step.status == 'fixed' ? c.brandPrimary : c.error),
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          );
        },
      );
    },
  );
}

void openLanShareSheet(BuildContext context, MlzAppState _, VpnController ctrl) {
  final c = Theme.of(context).extension<MlzColors>()!;
  bool usePin = true;

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: c.canvas,
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
    builder: (ctx) {
      return Consumer(
        builder: (ctx, ref, _) {
          final lan = ref.watch(vpnControllerProvider).lanShare;
          final qrData = 'http://${lan.username.isNotEmpty ? '${lan.username}:${lan.pin}@' : ''}${lan.primaryIp}:${lan.port}';

          return Directionality(
            textDirection: TextDirection.rtl,
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 14, 16, 24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text('اشتراک نقطه اتصال (LAN / Hotspot)', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                        Switch(
                          value: lan.enabled,
                          onChanged: (v) => ctrl.toggleLanSharing(enabled: v, usePin: usePin),
                        ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    SpatialGlassCard(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        children: [
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Text('آدرس پراکسی:', style: TextStyle(color: c.textMuted, fontSize: 12)),
                              Directionality(
                                textDirection: TextDirection.ltr,
                                child: Text('${lan.primaryIp}:${lan.port}', style: TextStyle(color: c.textPrimary, fontWeight: FontWeight.w700)),
                              ),
                            ],
                          ),
                          if (lan.pin.isNotEmpty) ...[
                            const SizedBox(height: 8),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Text('پین امنیتی:', style: TextStyle(color: c.textMuted, fontSize: 12)),
                                Directionality(
                                  textDirection: TextDirection.ltr,
                                  child: Text('${lan.username} / ${lan.pin}', style: TextStyle(color: c.brandPrimary, fontWeight: FontWeight.w700)),
                                ),
                              ],
                            ),
                          ],
                          const SizedBox(height: 14),
                          Container(
                            padding: const EdgeInsets.all(10),
                            decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(16)),
                            child: QrImageView(data: qrData, version: QrVersions.auto, size: 130),
                          ),
                          const SizedBox(height: 10),
                          TextButton.icon(
                            onPressed: () => Clipboard.setData(ClipboardData(text: '${lan.primaryIp}:${lan.port}')),
                            icon: const Icon(Icons.copy_rounded, size: 16),
                            label: const Text('کپی آدرس و پورت'),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      );
    },
  );
}

void openChangelogSheet(BuildContext context, MlzAppState _, VpnController ctrl) {
  final c = Theme.of(context).extension<MlzColors>()!;
  ctrl.checkForUpdates();

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: c.canvas,
    shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
    builder: (ctx) {
      return Consumer(
        builder: (ctx, ref, _) {
          final upd = ref.watch(vpnControllerProvider).availableUpdate;
          return Directionality(
            textDirection: TextDirection.rtl,
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text('به‌روزرسانی و تغییرات (GitHub Releases)', style: TextStyle(color: c.textPrimary, fontSize: 16, fontWeight: FontWeight.w800)),
                    const SizedBox(height: 10),
                    SpatialGlassCard(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            upd != null && upd.updateAvailable ? 'نسخه جدید ${upd.version} آماده است!' : 'نسخه فعلی ۱.۰.۰ (به‌روز است)',
                            style: TextStyle(color: c.brandPrimary, fontSize: 14, fontWeight: FontWeight.w800),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            upd != null && upd.changelogFA.isNotEmpty
                                ? upd.changelogFA.map((line) => '• $line').join('\n')
                                : '• هسته ترکیبی sing-box + Xray-core + پکیج کامل سایدکارها\n'
                                  '• تست Real Delay واقعی و مسیریابی مستقیم سایت‌های .ir\n'
                                  '• به‌روزرسانی خودکار ۵ دقیقه‌ای با تایید امضای Ed25519',
                            style: TextStyle(color: c.textMuted, fontSize: 12, height: 1.7),
                          ),
                          if (upd != null && upd.updateAvailable) ...[
                            const SizedBox(height: 12),
                            SizedBox(
                              width: double.infinity,
                              child: FilledButton.icon(
                                onPressed: ctrl.downloadAndInstallUpdate,
                                icon: const Icon(Icons.download_rounded, size: 18),
                                label: const Text('دانلود امن و نصب نسخه جدید'),
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      );
    },
  );
}
