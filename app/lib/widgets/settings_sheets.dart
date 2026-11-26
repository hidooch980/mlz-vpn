import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_ui/material_ui.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../screens/diagnostics_screen.dart';
import '../state/vpn_controller.dart';
import '../theme/mlz_theme.dart';

void openDiagnosticsSheet(
    BuildContext context, MlzAppState _, VpnController ctrl) {
  // The full-screen workspace runs the 9-step suite automatically and exposes
  // detailed findings plus one-tap auto-healing.
  Navigator.of(context).push(
    MaterialPageRoute<void>(builder: (_) => const DiagnosticsScreen()),
  );
}

void openLanShareSheet(
    BuildContext context, MlzAppState _, VpnController ctrl) {
  final c = Theme.of(context).extension<MlzColors>()!;
  bool usePin = true;

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: c.canvas,
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
    builder: (ctx) {
      return Consumer(
        builder: (ctx, ref, _) {
          final liveState = ref.watch(vpnControllerProvider);
          final lan = liveState.lanShare;
          final qrData =
              'http://${lan.username.isNotEmpty ? '${lan.username}:${lan.pin}@' : ''}${lan.primaryIp}:${lan.port}';

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
                        Text('اشتراک نقطه اتصال (LAN / Hotspot)',
                            style: TextStyle(
                                color: c.textPrimary,
                                fontSize: 16,
                                fontWeight: FontWeight.w800)),
                        Row(
                          children: [
                            if (liveState.lanBusy)
                              const Padding(
                                padding: EdgeInsetsDirectional.only(end: 8),
                                child: SizedBox(
                                  width: 16,
                                  height: 16,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 2),
                                ),
                              ),
                            Switch(
                              value: lan.enabled,
                              onChanged: liveState.lanBusy
                                  ? null
                                  : (v) => ctrl.toggleLanSharing(
                                      enabled: v, usePin: usePin),
                            ),
                          ],
                        ),
                      ],
                    ),
                    if (liveState.errorMessageFA != null) ...[
                      const SizedBox(height: 8),
                      Text(
                        liveState.errorMessageFA!,
                        textAlign: TextAlign.center,
                        style: TextStyle(color: c.error, fontSize: 12),
                      ),
                    ],
                    const SizedBox(height: 10),
                    SpatialGlassCard(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        children: [
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Text('آدرس پراکسی:',
                                  style: TextStyle(
                                      color: c.textMuted, fontSize: 12)),
                              Directionality(
                                textDirection: TextDirection.ltr,
                                child: Text('${lan.primaryIp}:${lan.port}',
                                    style: TextStyle(
                                        color: c.textPrimary,
                                        fontWeight: FontWeight.w700)),
                              ),
                            ],
                          ),
                          if (lan.pin.isNotEmpty) ...[
                            const SizedBox(height: 8),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Text('پین امنیتی:',
                                    style: TextStyle(
                                        color: c.textMuted, fontSize: 12)),
                                Directionality(
                                  textDirection: TextDirection.ltr,
                                  child: Text('${lan.username} / ${lan.pin}',
                                      style: TextStyle(
                                          color: c.brandPrimary,
                                          fontWeight: FontWeight.w700)),
                                ),
                              ],
                            ),
                          ],
                          const SizedBox(height: 14),
                          if (lan.enabled)
                            Container(
                              padding: const EdgeInsets.all(10),
                              decoration: BoxDecoration(
                                  color: Colors.white,
                                  borderRadius: BorderRadius.circular(16)),
                              child: QrImageView(
                                  data: qrData,
                                  version: QrVersions.auto,
                                  size: 130),
                            ),
                          const SizedBox(height: 10),
                          TextButton.icon(
                            onPressed: () =>
                                Clipboard.setData(ClipboardData(text: qrData)),
                            icon: const Icon(Icons.copy_rounded, size: 16),
                            label: const Text('کپی آدرس اتصال'),
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

void openChangelogSheet(
    BuildContext context, MlzAppState _, VpnController ctrl) {
  final c = Theme.of(context).extension<MlzColors>()!;
  ctrl.checkForUpdates();

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: c.canvas,
    shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28))),
    builder: (ctx) {
      return Consumer(
        builder: (ctx, ref, _) {
          final liveState = ref.watch(vpnControllerProvider);
          final upd = liveState.availableUpdate;
          return Directionality(
            textDirection: TextDirection.rtl,
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text('به‌روزرسانی و تغییرات (GitHub Releases)',
                        style: TextStyle(
                            color: c.textPrimary,
                            fontSize: 16,
                            fontWeight: FontWeight.w800)),
                    if (liveState.errorMessageFA != null) ...[
                      const SizedBox(height: 8),
                      Text(
                        liveState.errorMessageFA!,
                        style: TextStyle(color: c.error, fontSize: 12),
                      ),
                    ],
                    if (liveState.updateChecking)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: 14),
                        child: Center(child: CircularProgressIndicator()),
                      ),
                    const SizedBox(height: 10),
                    SpatialGlassCard(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            upd != null && upd.updateAvailable
                                ? 'نسخه جدید ${upd.version} آماده است!'
                                : 'نسخه فعلی ${liveState.appVersion} (به‌روز است)',
                            style: TextStyle(
                                color: c.brandPrimary,
                                fontSize: 14,
                                fontWeight: FontWeight.w800),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            upd != null && upd.changelogFA.isNotEmpty
                                ? upd.changelogFA
                                    .map((line) => '• $line')
                                    .join('\n')
                                : '• هسته ترکیبی sing-box + Xray-core + پکیج کامل سایدکارها\n'
                                    '• تست Real Delay واقعی و مسیریابی مستقیم سایت‌های .ir\n'
                                    '• به‌روزرسانی خودکار ۵ دقیقه‌ای با تایید امضای Ed25519',
                            style: TextStyle(
                                color: c.textMuted, fontSize: 12, height: 1.7),
                          ),
                          if (upd != null && upd.updateAvailable) ...[
                            const SizedBox(height: 12),
                            SizedBox(
                              width: double.infinity,
                              child: FilledButton.icon(
                                onPressed: liveState.updateDownloading
                                    ? null
                                    : ctrl.downloadAndInstallUpdate,
                                icon: liveState.updateDownloading
                                    ? const SizedBox(
                                        width: 16,
                                        height: 16,
                                        child: CircularProgressIndicator(
                                            strokeWidth: 2),
                                      )
                                    : const Icon(Icons.download_rounded,
                                        size: 18),
                                label: Text(liveState.updateDownloading
                                    ? 'در حال دانلود و بررسی امضا...'
                                    : 'دانلود امن و نصب نسخه جدید'),
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
