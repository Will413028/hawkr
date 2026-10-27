import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/block_model.dart';
import '../data/block_repository.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 封鎖名單列表頁
class BlockedUsersScreen extends ConsumerStatefulWidget {
  const BlockedUsersScreen({super.key});

  @override
  ConsumerState<BlockedUsersScreen> createState() => _BlockedUsersScreenState();
}

class _BlockedUsersScreenState extends ConsumerState<BlockedUsersScreen> {
  List<BlockedUser> _blocked = [];
  bool _isLoading = true;
  String? _errorMessage;
  final Set<String> _unblocking = {};

  @override
  void initState() {
    super.initState();
    _loadBlocked();
  }

  Future<void> _loadBlocked() async {
    try {
      final result = await ref.read(blockRepositoryProvider).getBlockedUsers();
      if (mounted) {
        setState(() {
          _blocked = result;
          _isLoading = false;
          _errorMessage = null;
        });
      }
    } catch (e) {
      if (!mounted) return;
      final message = e is AppException ? e.message : '載入失敗，請稍後再試';
      // 已有清單時（下拉刷新失敗）不整頁蓋掉，保留已顯示的資料改用 toast
      // 提示；只有初次載入（清單還是空的）才用整頁錯誤狀態接管。
      if (_blocked.isEmpty) {
        setState(() {
          _isLoading = false;
          _errorMessage = message;
        });
      } else {
        setState(() => _isLoading = false);
        showFloatingToast(context, message, isSuccess: false);
      }
    }
  }

  Future<void> _unblock(BlockedUser u) async {
    if (u.userId.isEmpty || _unblocking.contains(u.userId)) return;
    setState(() => _unblocking.add(u.userId));
    try {
      await ref.read(blockRepositoryProvider).unblockUser(u.userId);
      if (!mounted) return;
      setState(() {
        _blocked = _blocked.where((b) => b.userId != u.userId).toList();
        _unblocking.remove(u.userId);
      });
      showFloatingToast(context, '已解除封鎖', isSuccess: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _unblocking.remove(u.userId));
      showFloatingToast(
        context,
        e.toString().replaceAll('Exception: ', ''),
        isSuccess: false,
      );
    }
  }

  /// 讀取失敗狀態——與「真的沒資料」的空狀態分開顯示，避免使用者誤以為
  /// 沒有封鎖任何人；提供重試按鈕重新觸發 [_loadBlocked]。
  Widget _buildErrorState(String message) {
    return Center(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 32.w),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              AppIcons.errorOutline,
              color: AppIconColors.danger,
              size: 48.sp,
            ),
            SizedBox(height: 12.h),
            Text(
              message,
              textAlign: TextAlign.center,
              style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
            ),
            SizedBox(height: 12.h),
            TextButton(
              onPressed: _loadBlocked,
              child: const HkButtonLabel('重試'),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
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
            child: _isLoading
                ? const TruckLoadingOverlay(message: '載入中...')
                : _errorMessage != null
                ? _buildErrorState(_errorMessage!)
                : _blocked.isEmpty
                ? const HkEmptyState(
                    key: ValueKey('blocked-users-empty-state'),
                    iconKey: ValueKey('blocked-users-empty-state-icon'),
                    icon: HkIconName.emptyMerchants,
                    message: '尚無封鎖的使用者',
                  )
                : FruitRefreshIndicator(
                    onRefresh: _loadBlocked,
                    child: ListView.builder(
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
                      itemCount: _blocked.length,
                      itemBuilder: (_, i) => Padding(
                        padding: EdgeInsets.only(bottom: 12.h),
                        child: _buildBlockedRow(_blocked[i]),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildBlockedRow(BlockedUser u) {
    // /app/blocks 只回 id / blocked_id / create_time_ms，沒有名稱欄位
    // （見 BlockedUser 的欄位契約註解），一律用尾碼標示。
    final label =
        '使用者 ${u.userId.length > 6 ? u.userId.substring(u.userId.length - 6) : u.userId}';
    final isUnblocking = _unblocking.contains(u.userId);
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
            child: Text(
              label.isNotEmpty ? label[0].toUpperCase() : 'U',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: AppColors.primary,
              ),
            ),
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Text(
              label,
              style: TextStyle(fontSize: 16, color: AppColors.textPrimary),
              overflow: TextOverflow.ellipsis,
            ),
          ),
          TextButton(
            onPressed: isUnblocking ? null : () => _unblock(u),
            child: isUnblocking
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
