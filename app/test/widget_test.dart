import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:mlz_vpn/main.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('MLZ VPN renders cleanly on desktop and mobile with 2 tabs and drawers', (WidgetTester tester) async {
    SharedPreferences.setMockInitialValues({});

    // Test Windows desktop layout (920x640)
    tester.view.physicalSize = const Size(920, 640);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(const ProviderScope(child: MlzVpnApp()));
    await tester.pumpAndSettle();

    expect(find.text('خانه'), findsOneWidget);
    expect(find.text('تنظیمات'), findsOneWidget);
    expect(find.text('اتصال'), findsOneWidget);
    expect(find.text('تست آی‌پی'), findsOneWidget);
    expect(find.text('کشور اتصال'), findsOneWidget);
    expect(find.text('حالت / هسته'), findsOneWidget);
    expect(find.text('پینگ واقعی'), findsOneWidget);

    // Country sliding drawer
    await tester.tap(find.text('کشور اتصال'));
    await tester.pumpAndSettle();
    expect(find.text('انتخاب کشور (اتوماتیک یا دستی)'), findsOneWidget);
    await tester.tap(find.textContaining('آلمان').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('آلمان'), findsOneWidget);

    // Core/Auto Mode sliding drawer
    await tester.tap(find.text('حالت / هسته'));
    await tester.pumpAndSettle();
    expect(find.text('حالت اتصال (اتوماتیک یا بر اساس هسته‌ها)'), findsOneWidget);
    await tester.tap(find.text('ضدفیلتر Xray'));
    await tester.pumpAndSettle();
    expect(find.text('ضدفیلتر Xray'), findsOneWidget);

    // Settings screen (no overflow!)
    await tester.tap(find.text('تنظیمات'));
    await tester.pumpAndSettle();
    expect(find.text('عیب‌یابی هوشمند و خودترمیمی'), findsOneWidget);
    expect(find.text('اشتراک نقطه اتصال (LAN / Hotspot)'), findsOneWidget);
    expect(find.text('آپدیت از گیت‌هاب و Changelog'), findsOneWidget);

    // Also test mobile portrait width (390x844)
    tester.view.physicalSize = const Size(390, 844);
    await tester.pumpAndSettle();
    await tester.tap(find.text('خانه'));
    await tester.pumpAndSettle();
    expect(find.text('اتصال'), findsOneWidget);
  });
}
