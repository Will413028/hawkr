import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'package:hawkr_api/errors.dart' show errorText;

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/buyer_avatar_widget.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/store_block_repository.dart';
import 'report_list_controller.dart';
import 'widgets/block_actions.dart';

/// 封鎖名單頁（`ListBlockedBuyers`、`UnblockBuyer`）。
class BlockListScreen extends HookConsumerWidget {
  const BlockListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(blockListControllerProvider);
    final scrollController = useScrollController();
    final loadError = useState<Object?>(null);

    void loadMore() {
      loadError.value = null;
      ref
          .read(blockListControllerProvider.notifier)
          .loadMore()
          .then<void>(
            (_) {},
            onError: (Object e) {
              if (!context.mounted) return;
              final empty =
                  ref.read(blockListControllerProvider).value?.blocks.isEmpty ??
                  true;
              if (empty) {
                loadError.value = e;
                return;
              }
              FloatingToast.error(
                context,
                errorText(e, defaults: ref.read(defaultErrorTextProvider)),
              );
            },
          );
    }

    useEffect(() {
      void onScroll() {
        if (scrollController.position.pixels >=
            scrollController.position.maxScrollExtent - 200) {
          loadMore();
        }
      }

      scrollController.addListener(onScroll);
      return () => scrollController.removeListener(onScroll);
    }, [scrollController]);

    final loaded = asyncState.value;
    final blockCount = loaded?.blocks.length ?? 0;
    final pageToken = loaded?.nextPageToken ?? '';
    useEffect(() {
      if (pageToken.isEmpty) return null;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!context.mounted) return;
        final fills =
            !scrollController.hasClients ||
            scrollController.position.maxScrollExtent <= 0;
        if (fills) loadMore();
      });
      return null;
    }, [blockCount, pageToken]);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('封鎖名單'),
        onBackPressed: () => context.pop(),
      ),
      body: asyncState.when(
        loading: () => const TruckLoadingOverlay(),
        error: (error, _) => _buildErrorState(context, ref, error),
        data: (state) => FruitRefreshIndicator(
          // silentRefresh 失敗只跳 toast、保留既有清單（不整頁切成 error 狀態）
          onRefresh: () => ref
              .read(blockListControllerProvider.notifier)
              .silentRefresh()
              .catchError((Object e) {
                if (!context.mounted) return;
                FloatingToast.error(
                  context,
                  errorText(e, defaults: ref.read(defaultErrorTextProvider)),
                );
              }),
          child: state.blocks.isEmpty && !state.hasMore
              ? _buildEmptyState()
              : state.blocks.isEmpty && loadError.value != null
              ? ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  children: [
                    SizedBox(
                      height: 420.h,
                      child: _buildErrorState(
                        context,
                        ref,
                        loadError.value!,
                        onRetry: loadMore,
                      ),
                    ),
                  ],
                )
              : ListView.builder(
                  physics: const AlwaysScrollableScrollPhysics(),
                  controller: scrollController,
                  padding: EdgeInsets.symmetric(
                    horizontal: 16.w,
                    vertical: 12.h,
                  ),
                  itemCount:
                      state.blocks.length + (state.isLoadingMore ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == state.blocks.length) {
                      return Padding(
                        padding: EdgeInsets.all(16.w),
                        child: const Center(
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                      );
                    }
                    final blocked = state.blocks[index];
                    return _BlockedUserRow(
                      blocked: blocked,
                      onUnblock: () async {
                        final confirmed = await showUnblockConfirmDialog(
                          context,
                          targetName: blocked.displayLabel(context.uiLocale),
                        );
                        if (!confirmed) return;
                        if (!context.mounted) return;
                        try {
                          await ref
                              .read(blockListControllerProvider.notifier)
                              .unblock(blocked.buyerId);
                        } on Exception catch (e) {
                          if (!context.mounted) return;
                          FloatingToast.error(
                            context,
                            errorText(
                              e,
                              defaults: ref.read(defaultErrorTextProvider),
                            ),
                          );
                        }
                      },
                    );
                  },
                ),
        ),
      ),
    );
  }

  Widget _buildErrorState(
    BuildContext context,
    WidgetRef ref,
    Object error, {
    VoidCallback? onRetry,
  }) {
    final message = errorText(
      error,
      defaults: ref.read(defaultErrorTextProvider),
    );
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(AppIcons.errorOutline, size: 64.sp, color: AppIconColors.muted),
          SizedBox(height: 16.h),
          Text(
            message,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            textAlign: TextAlign.center,
          ),
          SizedBox(height: 16.h),
          ElevatedButton(
            onPressed:
                onRetry ??
                () => ref.read(blockListControllerProvider.notifier).refresh(),
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.primary),
            child: const HkButtonLabel(
              '重試',
              style: TextStyle(color: AppColors.bgCard),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: const [
        HkEmptyState(
          key: ValueKey('seller-block-list-empty-state'),
          iconKey: ValueKey('seller-block-list-empty-state-icon'),
          icon: HkIconName.search,
          message: '尚無封鎖名單',
          description: '你封鎖的使用者會顯示在這裡。',
          topPadding: 88,
        ),
      ],
    );
  }
}

class _BlockedUserRow extends StatelessWidget {
  const _BlockedUserRow({required this.blocked, required this.onUnblock});

  final BlockedBuyerItem blocked;
  final VoidCallback onUnblock;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: EdgeInsets.only(bottom: 12.h),
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.dividerLight),
      ),
      child: Row(
        children: [
          BuyerAvatarWidget(
            size: 36,
            imageUrl: blocked.avatarUrl,
            name: blocked.name,
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Text(
              blocked.displayLabel(context.uiLocale),
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w500,
                color: AppColors.textEmphasis,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          SizedBox(width: 8.w),
          OutlinedButton(
            onPressed: onUnblock,
            style: OutlinedButton.styleFrom(
              foregroundColor: AppColors.textPrimary,
              side: const BorderSide(color: AppColors.dividerLight),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(999.r),
              ),
            ),
            child: const HkButtonLabel('解除封鎖'),
          ),
        ],
      ),
    );
  }
}
