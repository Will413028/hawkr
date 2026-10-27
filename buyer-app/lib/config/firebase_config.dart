import 'package:firebase_core/firebase_core.dart'
    show Firebase, FirebaseApp, FirebaseException, FirebaseOptions;

/// Firebase 連線設定的編譯期來源：`--dart-define-from-file=config/dart-defines.json`
/// （範本見 `config/dart-defines.example.json`，實際檔案不進版控）。
const String _apiKey = String.fromEnvironment('FIREBASE_API_KEY');
const String _appId = String.fromEnvironment('FIREBASE_APP_ID');
const String _messagingSenderId = String.fromEnvironment(
  'FIREBASE_MESSAGING_SENDER_ID',
);
const String _projectId = String.fromEnvironment('FIREBASE_PROJECT_ID');
const String _storageBucket = String.fromEnvironment('FIREBASE_STORAGE_BUCKET');
const String _iosClientId = String.fromEnvironment('FIREBASE_IOS_CLIENT_ID');
const String _iosBundleId = String.fromEnvironment('FIREBASE_IOS_BUNDLE_ID');

/// 由欄位組出 [FirebaseOptions]；必填欄位（apiKey、appId、messagingSenderId、
/// projectId）任一為空白就回 `null`，代表 Firebase 沒有設定，呼叫端必須走停用路徑。
FirebaseOptions? buildFirebaseOptions({
  required String apiKey,
  required String appId,
  required String messagingSenderId,
  required String projectId,
  String storageBucket = '',
  String iosClientId = '',
  String iosBundleId = '',
}) {
  final required = [apiKey, appId, messagingSenderId, projectId];
  if (required.any((v) => v.trim().isEmpty)) return null;
  return FirebaseOptions(
    apiKey: apiKey,
    appId: appId,
    messagingSenderId: messagingSenderId,
    projectId: projectId,
    storageBucket: storageBucket.isEmpty ? null : storageBucket,
    iosClientId: iosClientId.isEmpty ? null : iosClientId,
    iosBundleId: iosBundleId.isEmpty ? null : iosBundleId,
  );
}

/// 編譯期 define 組出的設定；沒有提供時為 `null`。
final FirebaseOptions? definedFirebaseOptions = buildFirebaseOptions(
  apiKey: _apiKey,
  appId: _appId,
  messagingSenderId: _messagingSenderId,
  projectId: _projectId,
  storageBucket: _storageBucket,
  iosClientId: _iosClientId,
  iosBundleId: _iosBundleId,
);

/// Firebase 是否已初始化（[initializeFirebase] 成功，或測試注入了假平台）。
/// 推播、分析與背景處理等所有 Firebase 使用端以此決定走停用路徑。
bool get isFirebaseInitialized => Firebase.apps.isNotEmpty;

/// 依 [options] 初始化 default app 與名為 [appName] 的 app。[options] 為 `null`
/// （Firebase 沒有設定）時什麼都不做並回傳 `false`；成功回傳 `true`。
///
/// 熱重啟時 Dart 側的 apps 列表已清空、原生側仍保有舊實例，所以 duplicate-app
/// 視為已存在並取回。
Future<bool> initializeFirebase({
  required FirebaseOptions? options,
  required String appName,
}) async {
  if (options == null) return false;
  await _initApp(options: options);
  await _initApp(options: options, name: appName);
  return true;
}

Future<FirebaseApp> _initApp({
  required FirebaseOptions options,
  String? name,
}) async {
  try {
    return name != null
        ? await Firebase.initializeApp(name: name, options: options)
        : await Firebase.initializeApp(options: options);
  } on FirebaseException catch (e) {
    if (e.code != 'duplicate-app') rethrow;
    return name != null ? Firebase.app(name) : Firebase.app();
  }
}
