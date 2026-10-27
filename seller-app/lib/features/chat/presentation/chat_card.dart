import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
// 聊天室訊息「卡片 / 內嵌圖片」的共用渲染（單一來源）。
//
// 歷史上完整聊天室與已移除的貼文內嵌面板各自 copy-paste 一份
// looksLikeCard / maybeBuildCard / buildImageWidget。現在只保留這個共用版本，
// 避免卡片解析與圖片 fallback 再度分岔。
//
// looksLikeCard / maybeBuildCard / buildImageWidget 為 public；
// _buildCard / _buildOrderItem 僅供本檔內部使用，維持 private。
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../config/flavor.dart';
import '../../../config/image_config.dart';
import '../../media/data/media_repository.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/optimized_image.dart';

bool looksLikeCard(String content) {
  try {
    final parsed = json.decode(content);
    if (parsed is List) {
      final list = parsed.whereType<Map>().toList();
      return list.isNotEmpty;
    } else if (parsed is Map) {
      var map = Map<String, dynamic>.from(parsed);
      if (map['card'] is Map) {
        map = Map<String, dynamic>.from(map['card']);
      }
      final type = (map['type'] ?? '').toString().toLowerCase();
      final hasExplicitCard = type == 'card';
      final looksLikeCard =
          (map.containsKey('orders') ||
          map.containsKey('orderSn') ||
          map.containsKey('note') ||
          type.contains('order') ||
          (map.containsKey('title') &&
              (map.containsKey('subtitle') ||
                  map.containsKey('description') ||
                  map.containsKey('desc') ||
                  map.containsKey('image') ||
                  map.containsKey('imageUrl') ||
                  map.containsKey('price') ||
                  map.containsKey('orderNo') ||
                  map.containsKey('orderId') ||
                  map.containsKey('order_id'))));
      return hasExplicitCard || looksLikeCard;
    }
  } catch (_) {}
  return false;
}

Widget buildImageWidget(String content) {
  String s = content.trim();

  if (s.startsWith('{') || s.startsWith('[')) {
    try {
      final dynamic j = jsonDecode(s);
      String? pickFirst(dynamic v) {
        if (v is String) return v;
        if (v is List) {
          for (final e in v) {
            if (e is String && e.isNotEmpty) return e;
          }
        }
        return null;
      }

      if (j is Map) {
        const keys = [
          'url',
          'imageUrl',
          'image',
          'src',
          'path',
          'fileUrl',
          'downloadUrl',
        ];
        for (final k in keys) {
          final v = j[k];
          final got = pickFirst(v);
          if (got != null) {
            s = got.trim();
            break;
          }
        }
        if (s == content && j['images'] is List) {
          final got = pickFirst(j['images']);
          if (got != null) s = got.trim();
        }
        if (s == content && j['data'] is Map) {
          final d = j['data'] as Map;
          for (final k in ['url', 'image', 'imageUrl', 'src', 'path']) {
            final got = pickFirst(d[k]);
            if (got != null) {
              s = got.trim();
              break;
            }
          }
        }
      } else if (j is List) {
        final got = pickFirst(j);
        if (got != null) s = got.trim();
      }
    } catch (_) {}
  }

  if (s == content) {
    final md = RegExp(r'!\[[^\]]*\]\(([^)]+)\)');
    final m = md.firstMatch(s);
    if (m != null && m.groupCount >= 1) {
      s = m.group(1)!.trim();
    }
  }

  if (s.startsWith('data:image/')) {
    try {
      final cleaned = s.split(',').last.trim();
      final bytes = base64Decode(cleaned);
      return ClipRRect(
        borderRadius: BorderRadius.circular(8.r),
        child: Image.memory(
          bytes,
          width: 200.w,
          height: 200.h,
          fit: BoxFit.cover,
        ),
      );
    } catch (e) {
      HKLog.logD('[ChatThread] inline image decode error: $e');
    }
  }

  if (s.isNotEmpty && !s.startsWith('data:')) {
    String url;
    if (s.startsWith('http')) {
      url = s;
    } else if (s.startsWith('/')) {
      if (s.startsWith('/member/avatar') || s.startsWith('/member-avatars')) {
        url = MediaRepository.resolveUrl(s) ?? '';
      } else {
        // 其餘以 `/` 起頭者皆為 Gateway 路徑（`/public/*`、`/merchants/*`、
        // `/seller/*`、`/api/v1/chat/*`、`/chat/*`）——直接接 gateway host，
        // 不另組媒體 URL。
        url = '${F.appFlavor.gatewayHost}$s';
      }
    } else {
      // §231 S4：WS 圖片訊息 content 為裸 objectKey（如 public/chat/xxx.jpg）。
      // 直接交給 OptimizedNetworkImage 內部的 buildImageUrl（chat preset）組顯示 URL。
      url = s;
    }
    assert(() {
      HKLog.logD('[ChatThread] load image url: $url');
      return true;
    }());
    return ClipRRect(
      borderRadius: BorderRadius.circular(8.r),
      child: OptimizedNetworkImage(
        imageUrl: url,
        scene: ImageScene.chatImage,
        width: 200.w,
        height: 200.h,
        fit: BoxFit.cover,
        placeholder: Container(
          width: 200.w,
          height: 200.h,
          decoration: BoxDecoration(
            color: AppColors.gray200,
            borderRadius: BorderRadius.circular(8.r),
          ),
          child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
        ),
        errorWidget: Container(
          width: 200.w,
          height: 100.h,
          decoration: BoxDecoration(
            color: AppColors.gray200,
            borderRadius: BorderRadius.circular(8.r),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                AppIcons.imageNotSupported,
                color: AppIconColors.muted,
                size: 28.sp,
              ),
              SizedBox(height: 4.h),
              Text(
                '圖片已過期',
                style: TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 12.sp,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  try {
    final cleaned = s.split(',').last.trim();
    final bytes = base64Decode(cleaned);
    return ClipRRect(
      borderRadius: BorderRadius.circular(8.r),
      child: Image.memory(
        bytes,
        width: 200.w,
        height: 200.h,
        fit: BoxFit.cover,
      ),
    );
  } catch (e) {
    HKLog.logD('[ChatThread] image resolve failed: $e; content="$s"');
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(AppIcons.brokenImage, color: AppIconColors.danger),
        SizedBox(width: 6.w),
        Text('Image load failed'),
      ],
    );
  }
}

Widget? maybeBuildCard(String content) {
  try {
    final parsed = json.decode(content);
    if (parsed is List) {
      final list = parsed
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
      if (list.isNotEmpty) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (final m in list) ...[_buildCard(m), SizedBox(height: 8.h)],
          ],
        );
      }
    } else if (parsed is Map) {
      var map = Map<String, dynamic>.from(parsed);
      if (map['card'] is Map) {
        map = Map<String, dynamic>.from(map['card']);
      }
      final type = (map['type'] ?? '').toString().toLowerCase();
      final hasExplicitCard = type == 'card';
      final looksLikeCard =
          (map.containsKey('orders') ||
          map.containsKey('orderSn') ||
          map.containsKey('note') ||
          type.contains('order') ||
          (map.containsKey('title') &&
              (map.containsKey('subtitle') ||
                  map.containsKey('description') ||
                  map.containsKey('desc') ||
                  map.containsKey('image') ||
                  map.containsKey('imageUrl') ||
                  map.containsKey('price') ||
                  map.containsKey('orderNo') ||
                  map.containsKey('orderId') ||
                  map.containsKey('order_id'))));
      if (hasExplicitCard || looksLikeCard) {
        return _buildCard(map);
      }
    }
  } catch (_) {}
  return null;
}

Widget _buildCard(Map<String, dynamic> data) {
  final title = (data['title'] ?? '').toString();
  final subtitle =
      (data['subtitle'] ?? data['description'] ?? data['desc'] ?? '')
          .toString();
  final image = (data['image'] ?? data['imageUrl'] ?? '').toString();
  final price = data['price']?.toString();
  final List<Map<String, dynamic>> orders = (data['orders'] is List)
      ? List<Map<String, dynamic>>.from(
          (data['orders'] as List).whereType<Map>().map(
            (e) => Map<String, dynamic>.from(e),
          ),
        )
      : const <Map<String, dynamic>>[];
  final note = data['note']?.toString();

  if (orders.isNotEmpty) {
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: 260.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (title.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(bottom: 8.h),
              child: Text(
                title,
                style: const TextStyle(fontWeight: FontWeight.w700),
              ),
            ),
          for (final o in orders)
            Padding(
              padding: EdgeInsets.only(top: 8.h),
              child: _buildOrderItem(o),
            ),
        ],
      ),
    );
  }

  final status = data['status']?.toString();
  final orderNo =
      (data['orderNo'] ??
              data['orderSn'] ??
              data['order_id'] ??
              data['orderId'] ??
              '')
          .toString();
  final meta = data['meta']?.toString();
  final tags = (data['tags'] is List)
      ? List<String>.from((data['tags'] as List).map((e) => e.toString()))
      : const <String>[];

  return ConstrainedBox(
    constraints: BoxConstraints(maxWidth: 260.w),
    child: Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray300),
      ),
      padding: EdgeInsets.all(12.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (image.isNotEmpty)
            ClipRRect(
              borderRadius: BorderRadius.circular(8.r),
              child: OptimizedNetworkImage(
                imageUrl: image,
                scene: ImageScene.chatImage,
                width: 240.w,
                height: 120.h,
                fit: BoxFit.cover,
              ),
            ),
          if (image.isNotEmpty) SizedBox(height: 8.h),
          if (title.isNotEmpty)
            Text(title, style: const TextStyle(fontWeight: FontWeight.w700)),
          if (subtitle.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4.h),
              child: Text(
                subtitle,
                style: TextStyle(color: AppColors.textPrimary),
              ),
            ),
          if (orderNo.isNotEmpty ||
              (status != null && status.isNotEmpty) ||
              (price != null && price.isNotEmpty) ||
              tags.isNotEmpty)
            SizedBox(height: 8.h),
          if (orderNo.isNotEmpty)
            Row(
              children: [
                Icon(
                  AppIcons.receiptLong,
                  size: 14.sp,
                  color: AppIconColors.muted,
                ),
                SizedBox(width: 4.w),
                Flexible(
                  child: Text(
                    'Order: $orderNo',
                    style: TextStyle(fontSize: 12.sp),
                  ),
                ),
              ],
            ),
          if (status != null && status.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4.h),
              child: Row(
                children: [
                  Icon(
                    AppIcons.infoOutline,
                    size: 14.sp,
                    color: AppIconColors.muted,
                  ),
                  SizedBox(width: 4.w),
                  Flexible(
                    child: Text(status, style: TextStyle(fontSize: 12.sp)),
                  ),
                ],
              ),
            ),
          if (note != null && note.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 8.h),
              child: Row(
                children: [
                  Icon(
                    AppIcons.noteOutlined,
                    size: 14.sp,
                    color: AppIconColors.muted,
                  ),
                  SizedBox(width: 4.w),
                  Expanded(
                    child: Text(note, style: TextStyle(fontSize: 12.sp)),
                  ),
                ],
              ),
            ),
          if (price != null && price.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4.h),
              child: Row(
                children: [
                  Icon(
                    AppIcons.sellOutlined,
                    size: 14.sp,
                    color: AppIconColors.muted,
                  ),
                  SizedBox(width: 4.w),
                  Flexible(
                    child: Text(
                      'Price: $price',
                      style: TextStyle(fontSize: 12.sp),
                    ),
                  ),
                ],
              ),
            ),
          if (tags.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 8.h),
              child: Wrap(
                spacing: 6,
                runSpacing: -6,
                children: [
                  for (final t in tags)
                    Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: 8.w,
                        vertical: 4.h,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.gray200,
                        borderRadius: BorderRadius.circular(999.r),
                      ),
                      child: Text(t, style: TextStyle(fontSize: 12.sp)),
                    ),
                ],
              ),
            ),
          if (meta != null && meta.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 8.h),
              child: Text(
                meta,
                style: TextStyle(
                  fontSize: 12.sp,
                  color: AppColors.textSecondary,
                ),
              ),
            ),
        ],
      ),
    ),
  );
}

Widget _buildOrderItem(Map<String, dynamic> o) {
  final productName = (o['productName'] ?? o['name'] ?? '').toString();
  final date = (o['date'] ?? o['completeDate'] ?? o['completed_at'] ?? '')
      .toString();
  final remark = (o['remark'] ?? o['note'] ?? '').toString();
  final orderNo =
      (o['orderSn'] ?? o['orderNo'] ?? o['order_id'] ?? o['orderId'] ?? '')
          .toString();

  return Container(
    decoration: BoxDecoration(
      color: AppColors.bgCard,
      borderRadius: BorderRadius.circular(12.r),
      border: Border.all(color: AppColors.borderSubtle),
    ),
    padding: EdgeInsets.all(12.w),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (productName.isNotEmpty)
          Text(
            productName,
            style: TextStyle(fontSize: 14.sp, fontWeight: FontWeight.w700),
          ),
        if (date.isNotEmpty)
          Padding(
            padding: EdgeInsets.only(top: 8.h),
            child: Text(
              '完成日期：$date',
              style: TextStyle(fontSize: 12.sp, color: AppColors.textPrimary),
            ),
          ),
        if (orderNo.isNotEmpty)
          Padding(
            padding: EdgeInsets.only(top: 8.h),
            child: Text(
              '訂單編號：#$orderNo',
              style: TextStyle(fontSize: 12.sp, color: AppColors.textPrimary),
            ),
          ),
        if (remark.isNotEmpty)
          Padding(
            padding: EdgeInsets.only(top: 8.h),
            child: Text('備註：$remark', style: TextStyle(fontSize: 12.sp)),
          ),
      ],
    ),
  );
}
