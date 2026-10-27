import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/dialog_close_button.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../coupon/data/coupon_model.dart';
import '../data/customer_model.dart';
import '../data/customer_repository.dart';

/// 顧客詳情頁面返回結果
class CustomerDetailResult {
  final String customerId;
  final bool isVip;
  final bool isBlacklisted;

  const CustomerDetailResult({
    required this.customerId,
    required this.isVip,
    required this.isBlacklisted,
  });
}

class CustomerDetailScreen extends HookConsumerWidget {
  final Customer customer;

  const CustomerDetailScreen({super.key, required this.customer});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Full customer detail (enriched with email/address/avatar from detail API)
    final currentCustomer = useState<Customer>(customer);
    final updatingType = useState(false);

    // Local state for editing
    final currentTags = useState<List<CustomerTag>>(List.from(customer.tags));

    // Load fresh detail + tags on mount
    useEffect(() {
      Future<void> loadDetail() async {
        final repo = ref.read(customerRepositoryProvider);
        try {
          final [detail, tags] = await Future.wait([
            repo.getCustomerByAccountId(customer.id),
            repo.getCustomerTags(customer.id),
          ]);
          if (!context.mounted) return;
          if (detail is Customer) {
            currentCustomer.value = detail;
          }
          if (tags is List<CustomerTag>) {
            currentTags.value = tags;
          }
        } catch (_) {}
      }

      loadDetail();
      return null;
    }, []);

    // Use enriched customer for display (falls back to passed-in customer until detail loads)
    final c = currentCustomer.value;

    // Derive VIP/blacklist from customer_type (mutually exclusive)
    final isVip = c.customerType == 'vip';
    final isBlacklisted = c.customerType == 'blocklist';

    // Tag operations
    Future<void> addTag(String name) async {
      await ref.read(customerRepositoryProvider).addCustomerTags(customer.id, [
        name,
      ]);
      final fresh = await ref
          .read(customerRepositoryProvider)
          .getCustomerTags(customer.id);
      currentTags.value = fresh;
    }

    Future<void> removeTag(CustomerTag tag) async {
      await ref
          .read(customerRepositoryProvider)
          .removeCustomerTag(customer.id, tag.id);
      currentTags.value = currentTags.value
          .where((t) => t.id != tag.id)
          .toList();
    }

    void popWithResult() {
      context.pop(
        CustomerDetailResult(
          customerId: customer.id,
          isVip: isVip,
          isBlacklisted: isBlacklisted,
        ),
      );
    }

    Future<void> navigateToSpendingHistory({int initialTab = 0}) async {
      final repo = ref.read(customerRepositoryProvider);

      // Fetch with graceful fallback — endpoints may not exist yet (B-49)
      ({List<OrderRecord> orders, List<SpendingRecord> spending}) ordersRec = (
        orders: [],
        spending: [],
      );
      List<Coupon> couponsRec = [];

      try {
        ordersRec = await repo.getCustomerOrders(customer.id);
      } catch (_) {}

      try {
        couponsRec = await repo.getCustomerCoupons(customer.id);
      } catch (_) {}

      if (!context.mounted) return;

      int totalRef = 0;
      for (var s in ordersRec.spending) {
        totalRef += s.refund;
      }

      context.push(
        '/customer/spending-history',
        extra: {
          'customerName': c.name,
          'customerId': customer.id,
          'customerPhone': c.phone,
          'totalSpent': c.totalSpent,
          'totalRefund': totalRef,
          'records': ordersRec.spending,
          'orders': ordersRec.orders,
          'coupons': couponsRec,
          'initialTab': initialTab,
        },
      );
    }

    Future<void> showCustomerMenu(BuildContext buttonContext) async {
      if (updatingType.value) return;
      final button = buttonContext.findRenderObject()! as RenderBox;
      final overlay =
          Navigator.of(buttonContext).overlay!.context.findRenderObject()!
              as RenderBox;
      final value = await showMenu<String>(
        context: buttonContext,
        position: RelativeRect.fromRect(
          Rect.fromPoints(
            button.localToGlobal(Offset.zero, ancestor: overlay),
            button.localToGlobal(
              button.size.bottomRight(Offset.zero),
              ancestor: overlay,
            ),
          ),
          Offset.zero & overlay.size,
        ),
        items: [
          PopupMenuItem<String>(
            value: 'vip',
            child: Row(
              children: [
                Icon(
                  AppIcons.workspacePremiumOutlined,
                  size: 20.sp,
                  color: isVip
                      ? AppIconColors.warning
                      : AppIconColors.defaultColor,
                ),
                SizedBox(width: 8.w),
                Text(
                  isVip ? '取消 VIP' : '升級 VIP',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: isVip ? AppColors.primary : AppColors.textPrimary,
                  ),
                ),
              ],
            ),
          ),
          PopupMenuItem<String>(
            value: 'blacklist',
            child: Row(
              children: [
                Icon(
                  AppIcons.blockOutlined,
                  size: 20.sp,
                  color: isBlacklisted
                      ? AppIconColors.muted
                      : AppIconColors.defaultColor,
                ),
                SizedBox(width: 8.w),
                Text(
                  isBlacklisted ? '解除黑名單' : '設黑名單',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: isBlacklisted
                        ? AppColors.textSecondary
                        : AppColors.textPrimary,
                  ),
                ),
              ],
            ),
          ),
        ],
      );
      if (value == null || !context.mounted) return;
      if (updatingType.value) return;
      final newType = value == 'vip'
          ? (isVip ? 'normal' : 'vip')
          : (isBlacklisted ? 'normal' : 'blocklist');
      final message = value == 'vip'
          ? (isVip ? '已取消 VIP' : '已升級為 VIP')
          : (isBlacklisted ? '已解除黑名單' : '已加入黑名單');
      final repo = ref.read(customerRepositoryProvider);
      updatingType.value = true;
      try {
        await repo.updateCustomerType(customer.id, newType);
        if (!context.mounted) return;
        // Mutation success is authoritative even if the refresh is unavailable.
        currentCustomer.value = currentCustomer.value.copyWith(
          customerType: newType,
        );
        try {
          final detail = await repo.getCustomerByAccountId(customer.id);
          if (!context.mounted) return;
          if (detail != null) currentCustomer.value = detail;
        } catch (_) {}
        if (context.mounted) FloatingToast.success(context, message);
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '更新顧客狀態失敗，請稍後再試');
      } finally {
        if (context.mounted) updatingType.value = false;
      }
    }

    final moreButton = Builder(
      builder: (buttonContext) => HkFrostedIconButton(
        label: '顧客更多操作',
        icon: AppIcons.moreHoriz,
        onPressed: () => showCustomerMenu(buttonContext),
        loading: updatingType.value,
        loadingLabel: '正在更新顧客狀態',
      ),
    );

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) popWithResult();
      },
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: HkGlassAppBar(
          title: Text(c.name),
          subtitle: '顧客 360',
          onBackPressed: popWithResult,
          actions: [
            IconButton(
              tooltip: '聯絡顧客',
              onPressed: () => context.push(
                '/chat/thread',
                extra: {'title': c.name, 'participantId': c.id},
              ),
              icon: const HkIcon(
                HkIconName.chat,
                color: AppIconColors.defaultColor,
                semanticLabel: '聯絡顧客',
              ),
            ),
            moreButton,
          ],
        ),
        body: SingleChildScrollView(
          padding: EdgeInsets.symmetric(horizontal: 24.w),
          child: Column(
            children: [
              SizedBox(height: 16.h),
              _buildProfileSection(
                c,
                isVip,
                isBlacklisted,
                currentTags.value,
                context,
                onAddTag: addTag,
                onRemoveTag: removeTag,
              ),
              SizedBox(height: 16.h),
              Container(height: 1, color: const Color(0x1A42493F)),
              SizedBox(height: 16.h),
              _buildStatsSection(c, navigateToSpendingHistory),
              SizedBox(height: 16.h),
              _buildContactSection(c),
              SizedBox(height: 16.h),
              _buildFrequentItemsSection(c),
              SizedBox(height: 40.h),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildProfileSection(
    Customer customer,
    bool isVip,
    bool isBlacklisted,
    List<CustomerTag> tags,
    BuildContext context, {
    required Future<void> Function(String name) onAddTag,
    required Future<void> Function(CustomerTag tag) onRemoveTag,
  }) {
    return Column(
      children: [
        Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 100.w,
              height: 100.h,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: AppColors.textSecondary, width: 1),
              ),
              child: ClipOval(
                child: customer.avatarUrl != null
                    ? OptimizedNetworkImage(
                        imageUrl: customer.avatarUrl!,
                        scene: ImageScene.avatarLarge,
                        width: 100.w,
                        height: 100.h,
                        fit: BoxFit.cover,
                        errorWidget: _buildDefaultAvatar(),
                      )
                    : _buildDefaultAvatar(),
              ),
            ),
            if (isVip)
              Positioned(
                right: -4.w,
                bottom: -4.h,
                child: Container(
                  width: 32.w,
                  height: 32.h,
                  decoration: const BoxDecoration(
                    color: AppColors.warning,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: Icon(
                      AppIcons.workspacePremium,
                      size: 20.sp,
                      color: AppIconColors.onBrand,
                    ),
                  ),
                ),
              ),
            if (isBlacklisted)
              Positioned(
                bottom: -10.h,
                left: 0,
                right: 0,
                child: Center(
                  child: Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 12.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.textPrimary,
                      borderRadius: BorderRadius.circular(999.r),
                      border: Border.all(color: AppColors.bgCard, width: 1.5.w),
                      boxShadow: [
                        BoxShadow(
                          color: AppColors.cardShadow,
                          blurRadius: 4.r,
                          offset: Offset(0, 4.h),
                        ),
                      ],
                    ),
                    child: Text(
                      '黑名單',
                      style: TextStyle(
                        fontSize: 12.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.bgCard,
                        letterSpacing: 1.sp,
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ),
        SizedBox(height: 12.h),
        Text(
          customer.name,
          style: TextStyle(
            fontSize: 20.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textPrimary,
            height: 1.2,
          ),
        ),
        SizedBox(height: 8.h),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              'ID：${customer.id}',
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w400,
                color: AppColors.textPrimary,
                height: 1.44,
              ),
            ),
            SizedBox(width: 6.w),
            HkTapTarget(
              key: const ValueKey('customer-copy-id'),
              semanticLabel: '複製顧客 ID',
              hint: '將顧客 ID 複製到剪貼簿',
              tooltip: '複製顧客 ID',
              onTap: () {
                Clipboard.setData(ClipboardData(text: customer.id));
                FloatingToast.success(context, '已複製 ID');
              },
              child: Icon(AppIcons.copyOutlined, size: 20.w),
            ),
          ],
        ),
        SizedBox(height: 16.h),
        _buildTagsRow(
          tags,
          context,
          onAddTag: onAddTag,
          onRemoveTag: onRemoveTag,
        ),
      ],
    );
  }

  Widget _buildTagsRow(
    List<CustomerTag> tags,
    BuildContext context, {
    required Future<void> Function(String name) onAddTag,
    required Future<void> Function(CustomerTag tag) onRemoveTag,
  }) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      alignment: WrapAlignment.center,
      children: [
        ...tags.map(
          (tag) => _buildTagChip(tag, onRemove: () => onRemoveTag(tag)),
        ),
        HkTapTarget(
          key: const ValueKey('customer-manage-tags'),
          semanticLabel: '管理顧客標籤',
          hint: '新增或移除顧客標籤',
          tooltip: '管理顧客標籤',
          onTap: () =>
              _showTagManagementSheet(context, tags, onAddTag, onRemoveTag),
          child: Container(
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
            decoration: BoxDecoration(
              color: AppColors.primary,
              borderRadius: BorderRadius.circular(999.r),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(AppIcons.add, size: 16.sp, color: AppIconColors.onBrand),
                SizedBox(width: 4.w),
                Text(
                  '增加標籤',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w400,
                    color: AppColors.bgCard,
                    height: 1.43,
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildTagChip(CustomerTag tag, {required VoidCallback onRemove}) {
    return Container(
      padding: EdgeInsets.only(left: 12.w, right: 8.w, top: 4.h, bottom: 4.h),
      decoration: BoxDecoration(
        color: AppColors.primarySoft,
        borderRadius: BorderRadius.circular(999.r),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            AppIcons.labelOutline,
            size: 14.sp,
            color: AppIconColors.defaultColor,
          ),
          SizedBox(width: 4.w),
          Flexible(
            child: Text(
              tag.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w400,
                color: AppColors.textPrimary,
                height: 1.43,
              ),
            ),
          ),
          SizedBox(width: 4.w),
          HkTapTarget(
            key: ValueKey('customer-remove-tag-${tag.id}'),
            semanticLabel: '移除標籤 ${tag.name}',
            tooltip: '移除標籤 ${tag.name}',
            onTap: onRemove,
            child: Icon(
              AppIcons.close,
              size: 14.sp,
              color: AppIconColors.defaultColor,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDefaultAvatar() => Container(
    color: AppColors.gray300,
    child: Icon(AppIcons.person, size: 48.sp, color: AppIconColors.muted),
  );

  Widget _buildStatsSection(
    Customer customer,
    Function({int initialTab}) onNav,
  ) {
    return Column(
      children: [
        Row(
          children: [
            Expanded(
              child: _buildStatCard(
                icon: AppIcons.monetizationOnOutlined,
                label: '消費總金額',
                value: customer.totalSpent.toString(),
                onTap: () => onNav(),
              ),
            ),
            SizedBox(width: 16.w),
            Expanded(
              child: _buildStatCard(
                icon: AppIcons.assignmentOutlined,
                label: '訂單數',
                value: customer.visitCount.toString(),
                onTap: () => onNav(initialTab: 1),
              ),
            ),
          ],
        ),
        SizedBox(height: 16.h),
        Row(
          children: [
            Expanded(
              child: _buildStatCard(
                icon: AppIcons.couponOutlined,
                label: '優惠卷',
                value: customer.couponCount.toString(),
                onTap: () => onNav(initialTab: 2),
              ),
            ),
            const Spacer(),
          ],
        ),
      ],
    );
  }

  Widget _buildStatCard({
    required IconData icon,
    required String label,
    required String value,
    VoidCallback? onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 2,
              offset: Offset(0, 1),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                ExcludeSemantics(
                  child: Icon(icon, size: 24.w, color: AppIconColors.secondary),
                ),
                SizedBox(width: 4.w),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.brandPrimaryStrong,
                      height: 1.5,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
            SizedBox(height: 4.h),
            Text(
              value,
              style: TextStyle(
                fontSize: 20.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
                height: 1.4,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildContactSection(Customer customer) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 16.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '聯絡資訊',
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
              height: 1.5,
            ),
          ),
          SizedBox(height: 8.h),
          _buildContactRow(
            icon: AppIcons.phoneOutlined,
            value: customer.phone.isNotEmpty ? customer.phone : '未填寫',
          ),
          SizedBox(height: 8.h),
          _buildContactRow(
            icon: AppIcons.location,
            value: customer.address ?? '未填寫',
          ),
          SizedBox(height: 8.h),
          _buildContactRow(
            icon: AppIcons.emailOutlined,
            value: customer.email ?? '未填寫 Email',
          ),
        ],
      ),
    );
  }

  Widget _buildContactRow({required IconData icon, required String value}) {
    return Row(
      children: [
        ExcludeSemantics(
          child: Icon(icon, size: 24.w, color: AppIconColors.secondary),
        ),
        SizedBox(width: 6.w),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w400,
              color: AppColors.textPrimary,
              height: 1.5,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFrequentItemsSection(Customer customer) {
    if (customer.frequentItems.isEmpty) return const SizedBox.shrink();
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 16.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '常購品項',
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
              height: 1.5,
            ),
          ),
          SizedBox(height: 8.h),
          Wrap(
            spacing: 8.w,
            runSpacing: 8.h,
            children: customer.frequentItems.map((item) {
              return Container(
                padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(8.r),
                ),
                child: Text(
                  item,
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w400,
                    color: AppColors.textPrimary,
                    height: 1.43,
                  ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }

  void _showTagManagementSheet(
    BuildContext context,
    List<CustomerTag> currentTags,
    Future<void> Function(String) onAddTag,
    Future<void> Function(CustomerTag) onRemoveTag,
  ) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
      ),
      builder: (ctx) => _TagManagementSheet(
        initialTags: currentTags,
        onAddTag: onAddTag,
        onRemoveTag: onRemoveTag,
      ),
    );
  }
}

// ─── 管理客戶標籤 底部彈出頁 ─────────────────────────────────────────────────

/// 標籤管理底部彈窗的標題列。
///
/// 關閉按鈕的視覺維持右側錨點，外層仍提供至少 48dp 的點擊區。
class CustomerTagManagementHeader extends StatelessWidget {
  const CustomerTagManagementHeader({super.key, required this.onClose});

  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    return Stack(
      alignment: Alignment.center,
      children: [
        Text(
          '管理客戶標籤',
          style: TextStyle(
            fontSize: 18.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textPrimary,
          ),
        ),
        Positioned(
          right: 0,
          child: DialogCloseButton(
            onTap: onClose,
            visualAlignment: Alignment.centerRight,
          ),
        ),
      ],
    );
  }
}

class _TagManagementSheet extends HookWidget {
  final List<CustomerTag> initialTags;
  final Future<void> Function(String) onAddTag;
  final Future<void> Function(CustomerTag) onRemoveTag;

  const _TagManagementSheet({
    required this.initialTags,
    required this.onAddTag,
    required this.onRemoveTag,
  });

  static const _quickTags = ['奧客', '愛退貨', '高消費', '新客'];

  @override
  Widget build(BuildContext context) {
    final tags = useState(List<CustomerTag>.from(initialTags));
    final textCtrl = useTextEditingController();
    final inputText = useState('');
    final isLoading = useState(false);
    final hasChanges = useState(false);

    useEffect(() {
      void listener() => inputText.value = textCtrl.text.trim();
      textCtrl.addListener(listener);
      return () => textCtrl.removeListener(listener);
    }, [textCtrl]);

    Future<void> addTag(String name) async {
      final trimmed = name.trim();
      if (trimmed.isEmpty) return;
      if (tags.value.any(
        (t) => t.name.toLowerCase() == trimmed.toLowerCase(),
      )) {
        return;
      }
      isLoading.value = true;
      try {
        await onAddTag(trimmed);
        tags.value = [...tags.value, CustomerTag(id: 0, name: trimmed)];
        hasChanges.value = true;
        textCtrl.clear();
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '新增標籤失敗');
      } finally {
        isLoading.value = false;
      }
    }

    Future<void> removeTag(CustomerTag tag) async {
      if (tag.id == 0) {
        tags.value = tags.value.where((t) => t.name != tag.name).toList();
        hasChanges.value = true;
        return;
      }
      isLoading.value = true;
      try {
        await onRemoveTag(tag);
        tags.value = tags.value.where((t) => t.id != tag.id).toList();
        hasChanges.value = true;
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '移除標籤失敗');
      } finally {
        isLoading.value = false;
      }
    }

    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Drag handle
          Container(
            margin: EdgeInsets.only(top: 12.h),
            width: 40.w,
            height: 4.h,
            decoration: BoxDecoration(
              color: AppColors.dragHandle,
              borderRadius: BorderRadius.circular(2.r),
            ),
          ),
          Padding(
            padding: EdgeInsets.fromLTRB(20.w, 16.h, 20.w, 24.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                // Title + close button
                CustomerTagManagementHeader(
                  onClose: () => Navigator.pop(context),
                ),
                SizedBox(height: 20.h),

                // 目前標籤
                Text(
                  '目前標籤',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
                SizedBox(height: 8.h),
                if (tags.value.isEmpty)
                  Text(
                    '尚無標籤',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  )
                else
                  Wrap(
                    spacing: 8.w,
                    runSpacing: 8.h,
                    children: tags.value.map((tag) {
                      return HkTapTarget(
                        key: ValueKey('customer-sheet-remove-tag-${tag.id}'),
                        semanticLabel: '移除標籤 ${tag.name}',
                        tooltip: '移除標籤 ${tag.name}',
                        onTap: isLoading.value ? null : () => removeTag(tag),
                        child: Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 12.w,
                            vertical: 8.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.circular(999.r),
                            border: Border.all(color: AppColors.cardBorder),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Flexible(
                                child: Text(
                                  tag.name,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 14.sp,
                                    color: AppColors.textPrimary,
                                  ),
                                ),
                              ),
                              SizedBox(width: 6.w),
                              Icon(
                                AppIcons.close,
                                size: 14.sp,
                                color: AppIconColors.defaultColor,
                              ),
                            ],
                          ),
                        ),
                      );
                    }).toList(),
                  ),
                SizedBox(height: 16.h),

                // 新增標籤
                Text(
                  '新增標籤',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
                SizedBox(height: 8.h),
                Row(
                  children: [
                    Expanded(
                      child: Container(
                        height: 48,
                        decoration: BoxDecoration(
                          color: AppColors.gray100,
                          borderRadius: BorderRadius.circular(8.r),
                        ),
                        child: TextField(
                          controller: textCtrl,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textPrimary,
                          ),
                          decoration: InputDecoration(
                            hintText: '輸入標籤名稱',
                            hintStyle: TextStyle(
                              fontSize: 14.sp,
                              color: AppColors.textSecondary,
                            ),
                            contentPadding: EdgeInsets.symmetric(
                              horizontal: 12.w,
                            ),
                            border: InputBorder.none,
                            isCollapsed: true,
                          ),
                          textAlignVertical: TextAlignVertical.center,
                        ),
                      ),
                    ),
                    SizedBox(width: 8.w),
                    Opacity(
                      opacity: inputText.value.isEmpty || isLoading.value
                          ? 0.5
                          : 1.0,
                      child: HkTapTarget(
                        key: const ValueKey('customer-sheet-add-tag'),
                        semanticLabel: '新增顧客標籤',
                        hint: '新增輸入的標籤',
                        onTap: inputText.value.isEmpty || isLoading.value
                            ? null
                            : () => addTag(textCtrl.text),
                        child: Container(
                          height: 40.h,
                          padding: EdgeInsets.symmetric(horizontal: 16.w),
                          decoration: BoxDecoration(
                            color: AppColors.primary,
                            borderRadius: BorderRadius.circular(8.r),
                          ),
                          child: Center(
                            child: Text(
                              '新增',
                              style: TextStyle(
                                fontSize: 14.sp,
                                fontWeight: FontWeight.w700,
                                color: AppColors.bgCard,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                SizedBox(height: 16.h),

                // 快速標籤
                Text(
                  '快速標籤',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
                SizedBox(height: 8.h),
                Wrap(
                  spacing: 8.w,
                  runSpacing: 8.h,
                  children: _quickTags.map((name) {
                    final isAdded = tags.value.any(
                      (t) => t.name.toLowerCase() == name.toLowerCase(),
                    );
                    return HkTapTarget(
                      key: ValueKey('customer-quick-tag-$name'),
                      semanticLabel: '新增快速標籤 $name',
                      hint: isAdded ? '此標籤已新增' : '將此標籤加入顧客',
                      selected: isAdded,
                      onTap: isAdded || isLoading.value
                          ? null
                          : () => addTag(name),
                      child: Opacity(
                        opacity: isAdded ? 0.4 : 1.0,
                        child: Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 12.w,
                            vertical: 8.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primary,
                            borderRadius: BorderRadius.circular(999.r),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                AppIcons.add,
                                size: 14.sp,
                                color: AppIconColors.onBrand,
                              ),
                              SizedBox(width: 4.w),
                              Flexible(
                                child: Text(
                                  name,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 14.sp,
                                    color: AppColors.bgCard,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    );
                  }).toList(),
                ),
                SizedBox(height: 24.h),

                // 完成 button
                Opacity(
                  opacity: hasChanges.value ? 1.0 : 0.5,
                  child: HkTapTarget(
                    key: const ValueKey('customer-tag-management-done'),
                    semanticLabel: '完成標籤管理',
                    onTap: () => Navigator.pop(context),
                    child: Container(
                      width: double.infinity,
                      height: 48,
                      decoration: BoxDecoration(
                        color: AppColors.primary,
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                      child: Center(
                        child: Text(
                          '完成',
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.bgCard,
                          ),
                        ),
                      ),
                    ),
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
