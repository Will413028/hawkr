import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// 死端點防復發（backend-payuni#2215）。
///
/// ## 為什麼要有這個測試
///
/// App 曾有一批「打了但 gateway 上根本沒有」的呼叫。它們的共同形狀是
/// **錯誤被 catch 吞掉**，所以：編譯過、analyze 乾淨、單元測試綠、
/// 畫面也不報錯 —— 唯一的症狀是那件事從來沒發生過。
///
/// 代表案例 `MediaRepository.deleteFile` 打 `DELETE /api/v1/app/file-urls/delete`，
/// 每換一次頭像／封面就以為刪掉了舊圖，實際上 R2 物件永久累積，
/// 而 App 端完全無感（#2215 的成因）。
///
/// ## 這裡斷言什麼
///
/// 下面每一條都在 2026-08-05 對 dev gateway 實打過，回的是 gin 的
/// `404 page not found`（純文字，不是 envelope）—— 代表路由不存在，
/// 而不是權限或參數問題。同批的控制組（`GET /api/v1/app/sellers/me`）回 200，
/// 證明量測答得出肯定也答得出否定。
///
/// 清理的責任在後端：seller-service 於 finalize 新圖時 `ReleaseReferences`
/// 解除舊資產引用，media-service 的 GC 排程再回收 R2 物件。客戶端不需要、
/// 也不該有刪除端點。
void main() {
  /// gateway 上不存在的端點（2026-08-05 實打 dev 逐條確認回 404 page not found）。
  /// key = 端點字面值，value = 它原本假裝在做的事。
  const deadEndpoints = <String, String>{
    '/api/v1/app/file-urls/delete': 'MediaRepository.deleteFile（換圖後刪舊 R2 物件）',
    '/api/v1/app/media/finalize':
        'stagedToken 轉正（實際由 UpdateMyStore 的 staged token 欄位內含）',
    '/like_status': '查「我按過這則貼文沒」（實際欄位是列表 payload 的 is_liked）',
    '/like_count': '查貼文讚數（實際欄位是列表 payload 的 like_count）',
  };

  /// 還沒處置的死端點：**只准留在這一個檔案**，不准擴散。
  ///
  /// `POST /api/v1/app/logistics` 同樣是 404，但它和上面那批不同 ——
  /// `LogisticsTrackingScreen` 是產品規格書列出的畫面
  /// （`docs/design/SELLER_APP_PRODUCT_UX_MASTER_SPEC.md:322`），
  /// 而且 `OrderRoutes.logistics` 目前全 repo 零導航呼叫點，畫面根本打不開。
  /// 「接到活的 `GET /app/orders/{order_no}/logistics` 並補上入口」還是
  /// 「連同路由一起砍掉」是產品取捨，不由這一輪清理決定。
  ///
  /// 這條是 ratchet 不是豁免：改動任一邊測試都會轉紅，逼人回來更新這裡。
  const knownDeadPending = <String, String>{
    '/api/v1/app/logistics':
        'lib/features/order/data/payuni_logistics_service.dart',
  };

  Iterable<File> dartSources(String root) sync* {
    for (final entity in Directory(root).listSync(recursive: true)) {
      if (entity is File && entity.path.endsWith('.dart')) yield entity;
    }
  }

  /// 只留可執行的程式碼行。整行註解要排除，否則「解釋這些端點為何是死的」的
  /// 文件本身會讓測試轉紅 —— 那會逼著把成因從 codebase 裡刪掉，恰好相反。
  String codeOnly(String source) => source
      .split('\n')
      .where((line) => !line.trimLeft().startsWith('//'))
      .join('\n');

  test('lib/ 不得再出現任何已知的死端點', () {
    final offenders = <String>[];
    for (final file in dartSources('lib')) {
      final source = codeOnly(file.readAsStringSync());
      for (final entry in deadEndpoints.entries) {
        if (source.contains(entry.key)) {
          offenders.add('${file.path} → ${entry.key}（${entry.value}）');
        }
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          '這些端點在 gateway 上不存在，呼叫它們只會拿到 404 而且錯誤通常被吞掉。\n'
          '若後端真的補了對應路由，請先實打確認，再把該條目從本測試移除。\n'
          '命中：\n${offenders.join('\n')}',
    );
  });

  test('未處置的死端點不得擴散到第二個檔案', () {
    for (final entry in knownDeadPending.entries) {
      final hits = <String>[];
      for (final file in dartSources('lib')) {
        if (codeOnly(file.readAsStringSync()).contains(entry.key)) {
          hits.add(file.path);
        }
      }
      expect(
        hits,
        [entry.value],
        reason:
            '${entry.key} 在 gateway 上不存在。它目前只准出現在 ${entry.value}；\n'
            '若已接到活端點或整條砍掉，請把這個條目從 knownDeadPending 移除。',
      );
    }
  });

  test('MediaRepository 不得有任何 DELETE 呼叫', () {
    final source = codeOnly(
      File('lib/features/media/data/media_repository.dart').readAsStringSync(),
    );
    for (final forbidden in const ['.delete(', '.delete<', 'apiDelete(']) {
      expect(
        source,
        isNot(contains(forbidden)),
        reason:
            'media 資產的回收由後端負責（seller-service ReleaseReferences '
            '＋ media-service GC）。App 端沒有、也不該有刪除路徑：#2215 '
            '的整個成因就是這裡有一條假裝在刪、實際 404 的呼叫。',
      );
    }
  });
}
