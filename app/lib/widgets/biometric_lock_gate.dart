import 'package:local_auth/local_auth.dart';
import 'package:material_ui/material_ui.dart';
import '../theme/mlz_theme.dart';

/// Optional Biometric / device credential gate protecting app access when enabled.
class BiometricLockGate extends StatelessWidget {
  final VoidCallback onUnlocked;

  const BiometricLockGate({super.key, required this.onUnlocked});

  Future<void> _authenticate(BuildContext context) async {
    final auth = LocalAuthentication();
    try {
      final ok = await auth.authenticate(
        localizedReason: 'برای ورود به MLZ VPN احراز هویت کنید',
        options: const AuthenticationOptions(biometricOnly: false, stickyAuth: true),
      );
      if (ok) {
        onUnlocked();
      }
    } catch (_) {
      onUnlocked();
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = Theme.of(context).extension<MlzColors>()!;
    return Scaffold(
      backgroundColor: c.canvas,
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: SpatialGlassCard(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.fingerprint_rounded, size: 56, color: c.brandPrimary),
                const SizedBox(height: 14),
                Text('MLZ VPN قفل شده است', style: TextStyle(color: c.textPrimary, fontSize: 18, fontWeight: FontWeight.w800)),
                const SizedBox(height: 6),
                Text('برای مشاهده و مدیریت اتصال اثرانگشت یا رمز را تایید کنید', textAlign: TextAlign.center, style: TextStyle(color: c.textMuted, fontSize: 13)),
                const SizedBox(height: 18),
                FilledButton(
                  onPressed: () => _authenticate(context),
                  child: const Text('باز کردن قفل'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
