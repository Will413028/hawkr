import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    show StoreCard;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'package:hawkr_api/errors.dart' show errorText;
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../seller/data/store_card_display.dart';
import '../../social/presentation/write_result.dart';
import 'blocked_list_notifier.dart';
import 'paged_section.dart';
import 'paged_section_widgets.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 已刪除帳號的買家沒有名稱，名單上顯示這句，仍可解除封鎖。
const deletedAccountLabel = '已刪除的帳號';

/// 封鎖名單頁：「店家」「買家」兩段，各自分頁。伺服器狀態在 [blockedListProvider]；這裡只顯示與發動作。
class BlockedUsersScreen extends ConsumerWidget {
  const BlockedUsersScreen({super.key});

  static Object? _loadError(BlockedListState s) =>
      s.stores.error ?? s.buyers.error;

  void _toastError(BuildContext context, WidgetRef ref) {
    final error = _loadError(ref.read(blockedListProvider));
    if (error != null && isAccountSuspendedError(error)) return;
    showFloatingToast(context, _errorMessage(ref, error), isSuccess: false);
  }

  static String _errorMessage(WidgetRef ref, Object? error) => errorText(
    error ?? StateError('unknown'),
    defaults: ref.read(defaultErrorTextProvider),
  );

  Future<void> _reload(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(blockedListProvider.notifier).reload();
    if (!ok && context.mounted) {
      _toastError(context, ref);
    }
  }

  Future<void> _loadMore(
    BuildContext context,
    WidgetRef ref,
    Future<bool> Function() load,
  ) async {
    if (!await load() && context.mounted) {
      _toastError(context, ref);
    }
  }

  Future<void> _unblock(
    BuildContext context,
    WidgetRef ref,
    Future<WriteResult> Function() run,
  ) async {
    final result = await run();
    if (!context.mounted) return;
    switch (result.status) {
      case WriteStatus.applied:
        showFloatingToast(context, '已解除封鎖', isSuccess: true);
      case WriteStatus.failed:
        // 停權由 transport 的 interceptor 提示，這裡不再疊一則失敗訊息。
        final error = result.error;
        if (error == null || isAccountSuspendedError(error)) return;
        showFloatingToast(context, _errorMessage(ref, error), isSuccess: false);
      case WriteStatus.discarded:
        break;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final list = ref.watch(blockedListProvider);
    final notifier = ref.read(blockedListProvider.notifier);
    final locale = ref.watch(uiLocaleProvider);
    final stores = list.stores;
    final buyers = list.buyers;
    final failed =
        stores.phase == SectionPhase.failed &&
        buyers.phase == SectionPhase.failed;
    // 整頁 loader 只在兩段都還沒有任何結果時；之後一段的讀取或重試都在該段就地顯示 loader。
    final loading = _isPending(stores) && _isPending(buyers);
    final empty = !_hasContent(stores) && !_hasContent(buyers);
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          commonTitleTopBar(
            context,
            title: '封鎖名單',
            onBack: () => context.pop(),
          ),
          Expanded(
            child: loading
                ? const TruckLoadingOverlay(message: '載入中...')
                : failed
                ? PagedErrorState(
                    message: _errorMessage(ref, _loadError(list)),
                    onRetry: () => _reload(context, ref),
                  )
                : empty
                ? const HkEmptyState(
                    key: ValueKey('blocked-users-empty-state'),
                    iconKey: ValueKey('blocked-users-empty-state-icon'),
                    icon: HkIconName.emptyMerchants,
                    message: '尚無封鎖的使用者',
                  )
                : FruitRefreshIndicator(
                    onRefresh: () => _reload(context, ref),
                    child: ListView(
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
                      children: [
                        ..._section<buyer.BlockedStore>(
                          context,
                          ref,
                          title: '店家',
                          keyPrefix: 'blocked-stores',
                          section: stores,
                          onRetry: notifier.reloadStores,
                          onMore: notifier.loadMoreStores,
                          row: (store) {
                            final id = store.storeId;
                            final card = store.hasStore()
                                ? store.store
                                : StoreCard();
                            return _BlockedRow(
                              key: ValueKey('blocked-store-$id'),
                              name: card.displayName(locale),
                              note: card.statusLabel(locale),
                              avatarUri: card.logoUri,
                              unblocking: list.unblocking.contains(
                                storeUnblockKey(id),
                              ),
                              onUnblock: () => _unblock(
                                context,
                                ref,
                                () => notifier.unblockStore(id),
                              ),
                            );
                          },
                        ),
                        ..._section<buyer.BlockedBuyer>(
                          context,
                          ref,
                          title: '買家',
                          keyPrefix: 'blocked-buyers',
                          section: buyers,
                          onRetry: notifier.reloadBuyers,
                          onMore: notifier.loadMoreBuyers,
                          row: (b) {
                            final id = b.buyerId;
                            final name =
                                b.deleted || b.displayName.trim().isEmpty
                                ? deletedAccountLabel
                                : b.displayName.trim();
                            return _BlockedRow(
                              key: ValueKey('blocked-buyer-$id'),
                              name: name,
                              avatarUri: !b.deleted && b.hasAvatar()
                                  ? b.avatar.uri(ImagePreset.avatar)
                                  : '',
                              unblocking: list.unblocking.contains(
                                buyerUnblockKey(id),
                              ),
                              onUnblock: () => _unblock(
                                context,
                                ref,
                                () => notifier.unblockBuyer(id),
                              ),
                            );
                          },
                        ),
                      ],
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  /// 這一段此刻有東西可畫：有項目、讀取中（就地 loader）、讀取失敗（要給重試），或項目都被移走但還有下一頁。
  static bool _hasContent(PagedSection<Object?> s) =>
      s.items.isNotEmpty || s.phase != SectionPhase.ready || s.hasMore;

  /// 這一段還沒有任何結果、仍在第一次讀取。
  static bool _isPending(PagedSection<Object?> s) =>
      s.phase == SectionPhase.loading && s.items.isEmpty;

  /// 一段：標題、項目、「查看更多」；沒有項目也沒有失敗的段不顯示。
  List<Widget> _section<T>(
    BuildContext context,
    WidgetRef ref, {
    required String title,
    required String keyPrefix,
    required PagedSection<T> section,
    required Future<bool> Function() onRetry,
    required Future<bool> Function() onMore,
    required Widget Function(T item) row,
  }) {
    if (!_hasContent(section)) return const [];
    return [
      Padding(
        padding: EdgeInsets.only(bottom: 8.h),
        child: Text(
          title,
          key: ValueKey('$keyPrefix-title'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w700,
            color: AppColors.textSecondary,
          ),
        ),
      ),
      if (_isPending(section))
        Padding(
          padding: EdgeInsets.only(bottom: 12.h),
          child: Center(
            key: ValueKey('$keyPrefix-loading'),
            child: SizedBox(
              width: 20.w,
              height: 20.w,
              child: const HkBrandLoader(
                tone: HkBrandLoaderTone.mono,
                color: AppColors.primary,
              ),
            ),
          ),
        ),
      if (section.phase == SectionPhase.failed)
        TextButton(
          key: ValueKey('$keyPrefix-retry'),
          onPressed: onRetry,
          child: const HkButtonLabel('重試'),
        ),
      for (final item in section.items)
        Padding(
          padding: EdgeInsets.only(bottom: 12.h),
          child: row(item),
        ),
      if (section.hasMore)
        Padding(
          padding: EdgeInsets.only(bottom: 12.h),
          child: PagedLoadMoreFooter(
            moreKey: ValueKey('$keyPrefix-more'),
            loading: section.loadingMore,
            onMore: () => _loadMore(context, ref, onMore),
          ),
        ),
    ];
  }
}

class _BlockedRow extends StatelessWidget {
  const _BlockedRow({
    super.key,
    required this.name,
    required this.avatarUri,
    required this.unblocking,
    required this.onUnblock,
    this.note,
  });

  final String name;
  final String? note;
  final String avatarUri;
  final bool unblocking;
  final VoidCallback onUnblock;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Row(
        children: [
          CircleAvatar(
            radius: 18.r,
            backgroundColor: AppColors.gray100,
            backgroundImage: avatarUri.isEmpty ? null : NetworkImage(avatarUri),
            child: avatarUri.isEmpty
                ? Text(
                    name.isNotEmpty ? name[0].toUpperCase() : 'U',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primary,
                    ),
                  )
                : null,
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  style: TextStyle(fontSize: 16, color: AppColors.textPrimary),
                  overflow: TextOverflow.ellipsis,
                ),
                if (note != null)
                  Text(
                    note!,
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.textSecondary,
                    ),
                  ),
              ],
            ),
          ),
          TextButton(
            onPressed: unblocking ? null : onUnblock,
            child: unblocking
                ? SizedBox(
                    width: 16.w,
                    height: 16.w,
                    child: const HkBrandLoader(
                      tone: HkBrandLoaderTone.mono,
                      color: AppColors.primary,
                    ),
                  )
                : HkButtonLabel(
                    '解除封鎖',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: AppColors.danger,
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
