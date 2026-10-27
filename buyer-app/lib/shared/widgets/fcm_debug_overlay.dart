import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../core/routing/app_router.dart';
import '../../features/notification/data/push/manager.dart';
import 'hk_button_label.dart';

// ─────────────────────────────────────────────
// 資料模型
// ─────────────────────────────────────────────

class FcmDebugRecord {
  final DateTime receivedAt;
  final String? messageId;
  final String? fcmTitle;
  final String? fcmBody;
  final Map<String, dynamic> data;

  /// 消息来源：'onMessage'（前台）或 'onMessageOpenedApp'（后台点击/冷启动）
  final String source;

  const FcmDebugRecord({
    required this.receivedAt,
    this.messageId,
    this.fcmTitle,
    this.fcmBody,
    required this.data,
    required this.source,
  });

  String get refType =>
      (data['refType'] ?? data['type'] ?? 'unknown').toString();

  /// 将消息内容序列化为格式化 JSON 字符串
  String toJsonString() {
    final map = <String, dynamic>{
      'source': source,
      if (messageId != null) 'messageId': messageId,
      'notification': {
        if (fcmTitle != null) 'title': fcmTitle,
        if (fcmBody != null) 'body': fcmBody,
      },
      'data': data,
    };
    return const JsonEncoder.withIndent('  ').convert(map);
  }
}

// ─────────────────────────────────────────────
// 浮動按鈕（注入至 app.dart Stack）
// ─────────────────────────────────────────────

class FcmDebugOverlay extends StatelessWidget {
  const FcmDebugOverlay({super.key});

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<List<FcmDebugRecord>>(
      valueListenable: PushNotificationManager.instance.debugMessagesBus,
      builder: (context, records, _) {
        // 空資料時不讓 dev-only affordance 遮住商品、聊天或 Bottom Nav；收到
        // 真實 FCM debug record 後才顯示入口。
        if (records.isEmpty) {
          return const SizedBox.shrink();
        }

        return Positioned(
          right: 16,
          bottom: 100.h,
          child: GestureDetector(
            onTap: () => _openPanel(context, records),
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Container(
                  width: 56,
                  height: 56,
                  decoration: BoxDecoration(
                    color: AppColors.textEmphasis.withValues(alpha: 0.85),
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: AppColors.cardShadow,
                        blurRadius: 10,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  child: const Icon(
                    AppIcons.notificationsActiveOutlined,
                    color: AppIconColors.onBrand,
                    size: 28,
                  ),
                ),
                if (records.isNotEmpty)
                  Positioned(
                    right: -2,
                    top: -2,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 4,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.danger,
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Text(
                        records.length > 99 ? '99+' : '${records.length}',
                        style: const TextStyle(
                          color: AppColors.bgCard,
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _openPanel(BuildContext context, List<FcmDebugRecord> records) {
    // 使用 navigatorKey 的 context，避免 builder 層的 context 在 Navigator 之上
    final navContext = rootNavigatorKey.currentContext ?? context;
    showModalBottomSheet(
      context: navContext,
      isScrollControlled: true,
      showDragHandle: false,
      backgroundColor: Colors.transparent,
      builder: (_) => const FcmDebugPanel(),
    );
  }
}

// ─────────────────────────────────────────────
// Bottom Sheet 面板
// ─────────────────────────────────────────────

class FcmDebugPanel extends StatefulWidget {
  const FcmDebugPanel({super.key});

  @override
  State<FcmDebugPanel> createState() => _FcmDebugPanelState();
}

class _FcmDebugPanelState extends State<FcmDebugPanel> {
  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.6,
      minChildSize: 0.35,
      maxChildSize: 0.92,
      expand: false,
      builder: (context, scrollController) {
        return Container(
          decoration: const BoxDecoration(
            color: Color(0xFF1E1E1E),
            borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
          ),
          child: ValueListenableBuilder<List<FcmDebugRecord>>(
            valueListenable: PushNotificationManager.instance.debugMessagesBus,
            builder: (context, records, _) {
              return Column(
                children: [
                  _buildHandle(),
                  _buildHeader(context, records),
                  _buildTokenRow(context),
                  const Divider(color: AppColors.textEmphasis, height: 1),
                  Expanded(
                    child: records.isEmpty
                        ? _buildEmpty()
                        : ListView.separated(
                            controller: scrollController,
                            padding: const EdgeInsets.only(bottom: 24),
                            itemCount: records.length,
                            separatorBuilder: (_, __) => const Divider(
                              color: AppColors.textEmphasis,
                              height: 1,
                            ),
                            itemBuilder: (context, index) =>
                                _FcmDebugItem(record: records[index]),
                          ),
                  ),
                ],
              );
            },
          ),
        );
      },
    );
  }

  Widget _buildTokenRow(BuildContext context) {
    final token = PushNotificationManager.instance.fcmToken;
    if (token == null) return const SizedBox.shrink();

    return GestureDetector(
      onTap: () {
        Clipboard.setData(ClipboardData(text: token));
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('已复制 FCM Token'),
            duration: Duration(seconds: 1),
          ),
        );
      },
      child: Container(
        margin: const EdgeInsets.fromLTRB(16, 0, 16, 12),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: AppColors.textEmphasis),
        ),
        child: Row(
          children: [
            const Text(
              'FCM Token',
              style: TextStyle(
                color: Color(0xFF7EC8E3),
                fontSize: 12,
                fontFamily: 'monospace',
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                token,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: AppColors.gray300,
                  fontSize: 12,
                  fontFamily: 'monospace',
                ),
              ),
            ),
            const SizedBox(width: 6),
            const Icon(
              AppIcons.copyOutlined,
              size: 13,
              color: AppIconColors.muted,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHandle() {
    return Padding(
      padding: const EdgeInsets.only(top: 12, bottom: 4),
      child: Container(
        width: 36,
        height: 4,
        decoration: BoxDecoration(
          color: AppColors.textPrimary,
          borderRadius: BorderRadius.circular(2),
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, List<FcmDebugRecord> records) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 12, 12),
      child: Row(
        children: [
          const Icon(
            AppIcons.notificationsActiveOutlined,
            color: AppIconColors.secondary,
            size: 20,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'FCM Debug Log',
                  style: TextStyle(
                    color: AppColors.bgCard,
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                Text(
                  '_handleMessageOpenedApp  共 ${records.length} 則',
                  style: const TextStyle(
                    color: AppColors.textSecondary,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          if (records.isNotEmpty)
            TextButton(
              onPressed: () {
                PushNotificationManager.instance.debugMessagesBus.value = [];
              },
              style: TextButton.styleFrom(
                foregroundColor: const Color(0xFFFF6B6B),
                padding: const EdgeInsets.symmetric(horizontal: 8),
                minimumSize: const Size(48, 48),
              ),
              child: const HkButtonLabel('清除', style: TextStyle(fontSize: 14)),
            ),
          IconButton(
            onPressed: () => Navigator.of(context).pop(),
            icon: const Icon(
              AppIcons.close,
              color: AppIconColors.muted,
              size: 20,
            ),
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints(),
          ),
        ],
      ),
    );
  }

  Widget _buildEmpty() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            AppIcons.notificationsNoneOutlined,
            size: 48,
            color: AppIconColors.muted,
          ),
          const SizedBox(height: 12),
          Text(
            '尚未收到任何通知',
            style: TextStyle(color: AppColors.textSecondary, fontSize: 14),
          ),
          const SizedBox(height: 4),
          Text(
            '點擊推播通知後將在此顯示 payload',
            style: TextStyle(color: AppColors.textPrimary, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────
// 單筆記錄 Item — JSON 展示
// ─────────────────────────────────────────────

class _FcmDebugItem extends StatelessWidget {
  final FcmDebugRecord record;

  const _FcmDebugItem({required this.record});

  static const _sourceColors = <String, Color>{
    'onMessage': AppColors.success,
    'onMessageOpenedApp': Color(0xFF2196F3),
  };

  String _formatTime(DateTime dt) {
    final h = dt.hour.toString().padLeft(2, '0');
    final m = dt.minute.toString().padLeft(2, '0');
    final s = dt.second.toString().padLeft(2, '0');
    return '$h:$m:$s';
  }

  @override
  Widget build(BuildContext context) {
    final sourceColor = _sourceColors[record.source] ?? AppColors.textSecondary;
    final jsonStr = record.toJsonString();

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Header：来源标签 + 时间 + 复制按钮 ──
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: sourceColor.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(color: sourceColor.withValues(alpha: 0.5)),
                ),
                child: Text(
                  record.source,
                  style: TextStyle(
                    color: sourceColor,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    fontFamily: 'monospace',
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                _formatTime(record.receivedAt),
                style: const TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 12,
                  fontFamily: 'monospace',
                ),
              ),
              const Spacer(),
              // 复制 JSON 按钮
              GestureDetector(
                onTap: () {
                  Clipboard.setData(ClipboardData(text: jsonStr));
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(
                      content: Text('已复制 JSON'),
                      duration: Duration(seconds: 1),
                    ),
                  );
                },
                child: const Padding(
                  padding: EdgeInsets.all(4),
                  child: Icon(
                    AppIcons.copyOutlined,
                    size: 15,
                    color: AppIconColors.muted,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          // ── JSON 内容 ──
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: const Color(0xFF111111),
              borderRadius: BorderRadius.circular(6),
              border: Border.all(color: AppColors.textEmphasis),
            ),
            child: SelectableText(
              jsonStr,
              style: const TextStyle(
                color: AppColors.gray300,
                fontSize: 12,
                fontFamily: 'monospace',
                height: 1.5,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
