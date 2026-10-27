import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import 'notification_model.dart';

part 'notification_repository.g.dart';

@riverpod
NotificationRepository notificationRepository(Ref ref) {
  return NotificationRepository(ref.watch(dioProvider));
}

/// 通知 Repository
///
/// 封裝所有通知相關 API 呼叫。
/// 裝置管理 API 不在此 repository 中，未來歸 device feature。
class NotificationRepository {
  final Dio _dio;

  NotificationRepository(this._dio);

  static const _base = '/api/v1/app/notifications';

  // ============================================================
  // 通知列表
  // ============================================================

  /// 獲取通知列表（cursor-based 分頁）
  /// GET /api/v1/app/notifications?pageSize=20&cursorId=xxx
  /// Response: { code, data: { records: [...], total: "50", pageSize: "20", hasMore: true } }
  Future<NotificationListResult> getNotifications({
    int cursorId = 0,
    int size = 20,
  }) async {
    try {
      final params = <String, dynamic>{'pageSize': size};
      if (cursorId > 0) {
        params['cursorId'] = '$cursorId';
      }
      final response = await _dio.get(_base, queryParameters: params);

      final data = response.data;
      if (data['code'] == 0) {
        final rawData = data['data'];
        if (rawData is Map<String, dynamic>) {
          return NotificationListResult.fromJson(rawData);
        }
        return const NotificationListResult();
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '獲取通知列表失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// 獲取未讀數量
  /// GET /api/v1/app/notifications/unread-count
  /// Response: { code, data: { count: 5 } }
  Future<UnreadCount> getUnreadCount() async {
    try {
      final response = await _dio.get('$_base/unread-count');

      final data = response.data;
      if (data['code'] == 0) {
        final rawData = data['data'];
        if (rawData is Map<String, dynamic>) {
          return UnreadCount.fromJson(rawData);
        }
        return const UnreadCount();
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '獲取未讀數量失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  // ============================================================
  // 標記已讀
  // ============================================================

  /// 標記單個通知為已讀（冪等）
  /// PATCH /api/v1/app/notifications/{id}/read
  Future<void> markAsRead(int notificationId) async {
    try {
      final response = await _dio.patch('$_base/$notificationId/read');

      final data = response.data;
      if (data['code'] != 0) {
        throw AppException.fromApiError(
          data['code'] as int,
          data['message'] as String? ?? '標記已讀失敗',
        );
      }
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// 標記全部已讀
  /// PATCH /api/v1/app/notifications/read-all
  /// Response: { code, data: { updated: 12 } }
  Future<int> markAllAsRead() async {
    try {
      final response = await _dio.patch('$_base/read-all');

      final data = response.data;
      if (data['code'] == 0) {
        return data['data']?['updated'] as int? ?? 0;
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '標記全部已讀失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  // ============================================================
  // 刪除通知
  // ============================================================

  /// 批次刪除通知
  /// DELETE /api/v1/app/notifications
  /// Body: { "ids": [1, 2, 3] }
  /// Response: { code, data: { deleted: 3 } }
  Future<int> deleteNotifications(List<String> ids) async {
    if (ids.isEmpty) return 0;
    try {
      final response = await _dio.delete(_base, data: {'ids': ids});

      final data = response.data;
      if (data['code'] == 0) {
        final deleted = data['data']?['deleted'];
        return deleted is int
            ? deleted
            : (int.tryParse('$deleted') ?? ids.length);
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '刪除通知失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  // ============================================================
  // 偏好設定（賣家通知偏好）
  // ============================================================

  static const _prefBase = '/api/v1/app/sellers/me/notification-preferences';

  /// 獲取通知偏好設定
  /// GET /app/sellers/me/notification-preferences
  Future<List<NotificationPreferenceItem>> getPreferences() async {
    try {
      final response = await _dio.get(_prefBase);

      final data = response.data;
      if (data['code'] == 0) {
        final nested = data['data'];
        final list =
            (nested is Map ? nested['preferences'] : null) as List<dynamic>?;
        if (list == null) return [];
        return list
            .whereType<Map<String, dynamic>>()
            .map((e) => NotificationPreferenceItem.fromJson(e))
            .toList();
      }
      throw AppException.fromApiError(
        data['code'] as int,
        data['message'] as String? ?? '獲取偏好設定失敗',
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// 更新單一通知類別開關
  /// PUT /app/sellers/me/notification-preferences
  Future<void> updatePreference(String category, bool enabled) async {
    try {
      final response = await _dio.put(
        _prefBase,
        data: {'category': category, 'enabled': enabled},
      );

      final data = response.data;
      if (data['code'] != 0) {
        throw AppException.fromApiError(
          data['code'] as int,
          data['message'] as String? ?? '更新偏好設定失敗',
        );
      }
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }
}
