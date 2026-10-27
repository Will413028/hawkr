import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/chat_bubble_icon.dart';
import '../../../../shared/widgets/chat_input_bar.dart';
import '../../data/product_listing_model.dart';
import '../product_listing_draft_controller.dart';
import 'product_listing_chat_controller.dart';

/// 80% 高 bottom sheet 的對話式上架助手。
///
/// 上方：拖曳把手 + 標題 + 進度 badge。
/// 中段：訊息列表（user 右側綠 / assistant 左側白）。
/// 下方：重用的 [ChatInputBar]。
///
/// AI 透過 draft_patch SSE event 累積欄位到 [productListingDraftControllerProvider]，
/// 關閉此 sheet 後，各 step screen 會在 useEffect 套用 pending 值到 local state。
class ProductListingChatSheet extends HookConsumerWidget {
  const ProductListingChatSheet({super.key});

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      backgroundColor: Colors.transparent,
      builder: (_) => const ProductListingChatSheet(),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final inputController = useTextEditingController();
    final scrollController = useScrollController();
    final chatState = ref.watch(productListingChatControllerProvider);
    final draft = ref.watch(productListingDraftControllerProvider);

    // 新訊息進來時捲到最底
    ref.listen(productListingChatControllerProvider, (prev, next) {
      if (prev?.messages.length != next.messages.length ||
          (next.messages.isNotEmpty &&
              prev?.messages.lastOrNull?.text != next.messages.last.text)) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollController.hasClients) {
            scrollController.animateTo(
              scrollController.position.maxScrollExtent,
              duration: const Duration(milliseconds: 200),
              curve: Curves.easeOut,
            );
          }
        });
      }
      if (next.error != null && next.error != prev?.error) {
        FloatingToast.error(context, next.error!);
      }
    });

    final filledCount = draft.filledFields.length;
    const totalFields = 6; // name, origin, category, description, tags, skus

    return FractionallySizedBox(
      heightFactor: 0.85,
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.bgPrimary,
          borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
        ),
        child: Column(
          children: [
            _buildHeader(context, filledCount, totalFields),
            Expanded(
              child: chatState.messages.isEmpty
                  ? _buildEmptyState()
                  : ListView.builder(
                      controller: scrollController,
                      padding: EdgeInsets.symmetric(
                        horizontal: 16.w,
                        vertical: 12.h,
                      ),
                      itemCount:
                          chatState.messages.length +
                          (chatState.isStreaming &&
                                  chatState.messages.isNotEmpty &&
                                  chatState.messages.last.text.isEmpty
                              ? 1
                              : 0),
                      itemBuilder: (_, i) {
                        if (i >= chatState.messages.length) {
                          return _buildTypingIndicator();
                        }
                        return _MessageBubble(message: chatState.messages[i]);
                      },
                    ),
            ),
            ChatInputBar(
              controller: inputController,
              sending: chatState.isStreaming,
              hintText: chatState.isStreaming ? 'AI 正在回覆...' : '告訴 AI 你要上架的商品',
              onPlusPressed: () {}, // MVP 不支援附件
              onSend: () {
                final text = inputController.text;
                if (text.trim().isEmpty) return;
                inputController.clear();
                ref
                    .read(productListingChatControllerProvider.notifier)
                    .sendMessage(text);
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, int filled, int total) {
    return Container(
      padding: EdgeInsets.fromLTRB(16.w, 12.h, 8.w, 12.h),
      decoration: const BoxDecoration(
        border: Border(
          bottom: BorderSide(color: AppColors.cardBorder, width: 1),
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Center(
            child: Container(
              width: 40.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.gray300,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
          ),
          SizedBox(height: 10.h),
          Row(
            children: [
              Icon(
                AppIcons.autoAwesome,
                size: 18.sp,
                color: AppIconColors.secondary,
              ),
              SizedBox(width: 6.w),
              Text(
                'AI 上架助手',
                style: TextStyle(
                  fontSize: 16.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              SizedBox(width: 10.w),
              Container(
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(12.r),
                  border: Border.all(color: AppColors.primary),
                ),
                child: Text(
                  '已填 $filled / $total',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.primaryDark,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
              const Spacer(),
              IconButton(
                onPressed: () => Navigator.of(context).maybePop(),
                icon: Icon(
                  AppIcons.close,
                  size: 20.sp,
                  color: AppIconColors.defaultColor,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 40.h),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          ChatBubbleIcon(size: 48.sp, color: AppIconColors.secondary),
          SizedBox(height: 16.h),
          Text(
            '用對話讓 AI 幫你上架',
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 8.h),
          Text(
            '告訴 AI 你要賣什麼、產地、規格、價格，AI 會逐步引導你把商品欄位填完。\n關閉對話後，資料會自動帶入對應步驟。',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 14.sp,
              height: 1.6,
              color: AppColors.textSecondary,
            ),
          ),
          SizedBox(height: 16.h),
          Container(
            padding: EdgeInsets.all(12.r),
            decoration: BoxDecoration(
              color: AppColors.primarySoft,
              borderRadius: BorderRadius.circular(8.r),
            ),
            child: Text(
              '例：「我要賣台灣新鮮香蕉，一串 1 公斤 100 元」',
              style: TextStyle(fontSize: 12.sp, color: AppColors.primaryDark),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTypingIndicator() {
    return Padding(
      padding: EdgeInsets.only(left: 8.w, bottom: 8.h),
      child: Row(
        children: [
          Container(
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(16.r),
              border: Border.all(color: AppColors.cardBorder),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: 12.w,
                  height: 12.w,
                  child: const CircularProgressIndicator(
                    strokeWidth: 2,
                    color: AppColors.primary,
                  ),
                ),
                SizedBox(width: 8.w),
                Text(
                  'AI 思考中...',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({required this.message});
  final ChatMessage message;

  @override
  Widget build(BuildContext context) {
    final isSelf = message.role == 'user';
    if (message.text.isEmpty && !isSelf) return const SizedBox.shrink();

    return Padding(
      padding: EdgeInsets.only(bottom: 12.h),
      child: Row(
        mainAxisAlignment: isSelf
            ? MainAxisAlignment.end
            : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ConstrainedBox(
            constraints: BoxConstraints(maxWidth: 260.w),
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
              decoration: BoxDecoration(
                color: isSelf ? AppColors.primary : AppColors.bgCard,
                borderRadius: BorderRadius.circular(isSelf ? 20.r : 16.r),
                border: isSelf ? null : Border.all(color: AppColors.cardBorder),
              ),
              child: Text(
                message.text,
                style: TextStyle(
                  fontSize: 14.sp,
                  height: 1.5,
                  color: isSelf ? AppColors.bgPrimary : AppColors.textPrimary,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
