import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 聊天列表空狀態。
///
/// [isSearching] 為 true 時切換為搜尋空狀態；一般情況使用訊息語意圖示，
/// 兩者共用相同 halo 與排版。
class ChatEmptyState extends StatelessWidget {
  const ChatEmptyState({
    super.key,
    required this.isSearching,
    this.onClearSearch,
  });

  final bool isSearching;
  final VoidCallback? onClearSearch;

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(
        HkSpace.lg,
        HkSpace.sm,
        HkSpace.lg,
        HkSpace.section,
      ),
      children: [
        HkContentCard(
          key: const ValueKey('seller-chat-empty-state'),
          elevated: true,
          padding: const EdgeInsets.fromLTRB(
            HkSpace.xxl,
            HkSpace.xl,
            HkSpace.xxl,
            HkSpace.xxl,
          ),
          child: Column(
            children: [
              HkEmptyState(
                iconKey: const ValueKey('seller-chat-empty-state-icon'),
                icon: isSearching ? HkIconName.search : HkIconName.chat,
                message: isSearching ? '找不到相關對話' : '還沒有顧客對話',
                description: isSearching
                    ? '試試顧客名稱或訊息關鍵字，也可以清除搜尋重新查看全部對話。'
                    : '顧客從商品或訂單聯絡您後，對話會出現在這裡；需要營運協助可使用上方 AI 助手。',
                topPadding: 0,
              ),
              if (isSearching && onClearSearch != null) ...[
                const SizedBox(height: HkSpace.lg),
                OutlinedButton.icon(
                  onPressed: onClearSearch,
                  icon: const HkIcon(HkIconName.close, size: 20),
                  label: const HkButtonLabel('清除搜尋'),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}
