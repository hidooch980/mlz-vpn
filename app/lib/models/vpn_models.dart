enum VpnConnectionState {
  disconnected,
  connecting,
  connected,
  healing,
  error;

  static VpnConnectionState fromWire(String? raw) {
    switch (raw) {
      case 'connecting':
        return VpnConnectionState.connecting;
      case 'connected':
        return VpnConnectionState.connected;
      case 'healing':
        return VpnConnectionState.healing;
      case 'error':
        return VpnConnectionState.error;
      default:
        return VpnConnectionState.disconnected;
    }
  }
}

enum TunnelModeOption {
  tun('tun', 'TUN کل سیستم (درخواست ادمین در ویندوز)'),
  proxy('proxy', 'پراکسی سبک (بدون نیاز به ادمین)');

  final String wireValue;
  final String labelFA;
  const TunnelModeOption(this.wireValue, this.labelFA);
}

enum EngineModeOption {
  auto(
      'auto', 'اتوماتیک هوشمند', 'انتخاب هوشمند بهترین سرور و مسیر (پیشنهادی)'),
  singboxFast('singbox_fast', 'سرعت و گیمینگ',
      'هسته sing-box: Hysteria2 / TUIC / WireGuard'),
  xrayAntiDpi(
      'xray_antidpi', 'ضدفیلتر Xray', 'هسته Xray-core: XHTTP + TLS Fragment'),
  sidecarModern('sidecar_modern', 'تونل‌های مقاوم مدرن',
      'AmneziaWG • usque MASQUE • Naive • Cloak'),
  sidecarEmergency('sidecar_emergency', 'شبکه‌های اضطراری',
      'Lyrebird (Snowflake) • Tor/Arti • Psiphon • Lantern'),
  dpiDesync('dpi_desync', 'شکن DPI و پورت بسته',
      'ByeDPI • SpoofDPI • dnstt (تونل DNS) • Brook/Gost'),
  batterySaver('battery_saver', 'صرفه‌جویی باتری',
      'تک‌هسته کم‌مصرف + کاهش تست پس‌زمینه');

  final String wireValue;
  final String titleFA;
  final String subtitleFA;
  const EngineModeOption(this.wireValue, this.titleFA, this.subtitleFA);

  /// Android ships only the sing-box/libbox dataplane; Xray and process
  /// sidecars are desktop-only and must not be offered as working modes.
  bool get supportedOnAndroid =>
      this == EngineModeOption.auto ||
      this == EngineModeOption.singboxFast ||
      this == EngineModeOption.batterySaver;
}

class CountryChoice {
  final String code; // "auto" or ISO 2-letter code
  final String nameFA;
  final String flag;
  final String hintPing;

  const CountryChoice({
    required this.code,
    required this.nameFA,
    required this.flag,
    required this.hintPing,
  });

  bool get isAuto => code == 'auto';

  static const presets = <CountryChoice>[
    CountryChoice(
        code: 'auto',
        nameFA: 'اتوماتیک (بهترین کشور)',
        flag: '🌐',
        hintPing: 'خودکار'),
    CountryChoice(code: 'DE', nameFA: 'آلمان', flag: '🇩🇪', hintPing: '42 ms'),
    CountryChoice(code: 'NL', nameFA: 'هلند', flag: '🇳🇱', hintPing: '45 ms'),
    CountryChoice(
        code: 'FI', nameFA: 'فنلاند', flag: '🇫🇮', hintPing: '49 ms'),
    CountryChoice(
        code: 'FR', nameFA: 'فرانسه', flag: '🇫🇷', hintPing: '54 ms'),
    CountryChoice(code: 'CH', nameFA: 'سوئیس', flag: '🇨🇭', hintPing: '58 ms'),
    CountryChoice(
        code: 'GB', nameFA: 'انگلیس', flag: '🇬🇧', hintPing: '63 ms'),
    CountryChoice(
        code: 'US', nameFA: 'آمریکا', flag: '🇺🇸', hintPing: '128 ms'),
    CountryChoice(code: 'TR', nameFA: 'ترکیه', flag: '🇹🇷', hintPing: '38 ms'),
  ];
}

class IpTestResultModel {
  final String publicIp;
  final String countryCode;
  final String countryNameFA;
  final int realDelayMs;
  final bool dnsProtected;
  final bool iranDirectOk;

  const IpTestResultModel({
    required this.publicIp,
    required this.countryCode,
    required this.countryNameFA,
    required this.realDelayMs,
    required this.dnsProtected,
    required this.iranDirectOk,
  });

  factory IpTestResultModel.fromJson(Map<String, dynamic> j) {
    return IpTestResultModel(
      publicIp: (j['public_ip'] as String?) ?? '--',
      countryCode: (j['country_code'] as String?) ?? '',
      countryNameFA: (j['country_name_fa'] as String?) ?? 'نامشخص',
      realDelayMs: (j['real_delay_ms'] as num?)?.toInt() ?? 0,
      dnsProtected: (j['dns_protected'] as bool?) ?? false,
      iranDirectOk: (j['iran_direct_ok'] as bool?) ?? true,
    );
  }
}

class DiagnosticStepModel {
  final String id;
  final String titleFA;
  final String status;
  final String severity;
  final String detailFA;
  final bool autoApplied;
  final bool canAutoFix;
  final String fixLabelFA;

  const DiagnosticStepModel({
    required this.id,
    required this.titleFA,
    required this.status,
    required this.severity,
    required this.detailFA,
    required this.autoApplied,
    required this.canAutoFix,
    required this.fixLabelFA,
  });

  bool get isFailed => status == 'failed';
  bool get isFixed => status == 'fixed';
  bool get isInfo => status == 'info';

  factory DiagnosticStepModel.fromJson(Map<String, dynamic> j) {
    return DiagnosticStepModel(
      id: (j['id'] as String?) ?? '',
      titleFA: (j['title_fa'] as String?) ?? '',
      status: (j['status'] as String?) ?? 'ok',
      severity: (j['severity'] as String?) ?? 'ok',
      detailFA: (j['detail_fa'] as String?) ?? '',
      autoApplied: (j['auto_applied'] as bool?) ?? false,
      canAutoFix: (j['can_auto_fix'] as bool?) ?? false,
      fixLabelFA: (j['fix_label_fa'] as String?) ?? '',
    );
  }
}

class DiagnosticsReportModel {
  final String summaryFA;
  final int timestamp;
  final String headlineFA;
  final String healthLabelFA;
  final int healthScore;
  final bool autoHealed;
  final int autoFixCount;
  final bool needsAction;
  final List<DiagnosticStepModel> steps;

  const DiagnosticsReportModel({
    required this.summaryFA,
    required this.timestamp,
    required this.headlineFA,
    required this.healthLabelFA,
    required this.healthScore,
    required this.autoHealed,
    required this.autoFixCount,
    required this.needsAction,
    required this.steps,
  });

  bool get hasAutoFixable =>
      steps.any((step) => step.isFailed && step.canAutoFix);

  factory DiagnosticsReportModel.fromJson(Map<String, dynamic> j) {
    final rawSteps = (j['steps'] as List<dynamic>?) ?? const [];
    return DiagnosticsReportModel(
      summaryFA: (j['summary_fa'] as String?) ?? '',
      timestamp: (j['timestamp'] as num?)?.toInt() ?? 0,
      headlineFA: (j['headline_fa'] as String?) ?? '',
      healthLabelFA: (j['health_label_fa'] as String?) ?? '',
      healthScore: (j['health_score'] as num?)?.toInt() ?? 0,
      autoHealed: (j['auto_healed'] as bool?) ?? false,
      autoFixCount: (j['auto_fix_count'] as num?)?.toInt() ?? 0,
      needsAction: (j['needs_action'] as bool?) ?? false,
      steps: rawSteps
          .map((e) => DiagnosticStepModel.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}

class LanShareModel {
  final bool enabled;
  final String primaryIp;
  final int port;
  final String username;
  final String pin;
  final bool firewallManaged;

  const LanShareModel({
    required this.enabled,
    required this.primaryIp,
    required this.port,
    required this.username,
    required this.pin,
    required this.firewallManaged,
  });

  factory LanShareModel.fromJson(Map<String, dynamic> j) {
    return LanShareModel(
      enabled: (j['enabled'] as bool?) ?? false,
      primaryIp: (j['primary_ip'] as String?) ?? '192.168.137.1',
      port: (j['port'] as num?)?.toInt() ?? 10808,
      username: (j['username'] as String?) ?? '',
      pin: (j['pin'] as String?) ?? '',
      firewallManaged: (j['firewall_managed'] as bool?) ?? false,
    );
  }
}

class AppUpdateInfo {
  final bool updateAvailable;
  final String version;
  final List<String> changelogFA;
  final String downloadUrl;
  final String sha256;

  const AppUpdateInfo({
    required this.updateAvailable,
    required this.version,
    required this.changelogFA,
    required this.downloadUrl,
    required this.sha256,
  });
}
