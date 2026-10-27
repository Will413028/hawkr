import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// User-selectable text size layered on top of the platform accessibility
/// preference.
///
/// This is deliberately a text-only adjustment. Layout dimensions, images,
/// icons and tap targets stay in logical pixels so changing this setting never
/// scales the whole interface.
enum AppTextSize {
  small(label: '小', scaleAdjustment: -0.10),
  standard(label: '中', scaleAdjustment: 0),
  large(label: '大', scaleAdjustment: 0.15);

  const AppTextSize({required this.label, required this.scaleAdjustment});

  final String label;

  /// Additive adjustment applied to the effective platform text scale.
  /// Additive scaling keeps nonlinear Dynamic Type behavior intact.
  final double scaleAdjustment;

  static AppTextSize fromStorage(String? value) {
    return AppTextSize.values.firstWhere(
      (candidate) => candidate.name == value,
      orElse: () => AppTextSize.standard,
    );
  }
}

final appTextSizeProvider =
    NotifierProvider<AppTextSizeController, AppTextSize>(
      AppTextSizeController.new,
    );

class AppTextSizeController extends Notifier<AppTextSize> {
  static const storageKey = 'hk_app_text_size_v1';

  var _writeRevision = 0;
  var _persistedState = AppTextSize.standard;
  Future<void> _writeTail = Future<void>.value();

  @override
  AppTextSize build() => AppTextSize.standard;

  /// Restores the device-level preference before the first application frame.
  Future<void> restore() async {
    try {
      final preferences = await SharedPreferences.getInstance();
      state = AppTextSize.fromStorage(preferences.getString(storageKey));
      _persistedState = state;
    } catch (_) {
      // A display preference must never block startup.
      state = AppTextSize.standard;
      _persistedState = state;
    }
  }

  /// Applies immediately and persists for future launches.
  ///
  /// Returns false when persistence fails and rolls back the latest attempted
  /// change so the visible state never promises a value that was not saved.
  Future<bool> setTextSize(AppTextSize value) async {
    if (value == state) return true;

    final revision = ++_writeRevision;
    state = value;

    final operation = _writeTail.then((_) async {
      // A newer tap already superseded this value before its write started.
      if (revision != _writeRevision) return true;
      try {
        final preferences = await SharedPreferences.getInstance();
        final saved = await preferences.setString(storageKey, value.name);
        if (saved) {
          _persistedState = value;
        } else if (revision == _writeRevision) {
          state = _persistedState;
        }
        return saved;
      } catch (_) {
        if (revision == _writeRevision) state = _persistedState;
        return false;
      }
    });
    _writeTail = operation.then<void>((_) {});
    return operation;
  }
}
