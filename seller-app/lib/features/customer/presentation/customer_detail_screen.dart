import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
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
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/dialog_close_button.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../data/customer_model.dart';
import '../data/customer_repository.dart';

/// 顧客詳情頁面返回結果
class CustomerDetailResult {
  final String customerId;
  final bool isVip;

  const CustomerDetailResult({required this.customerId, required this.isVip});
}

class CustomerDetailScreen extends HookConsumerWidget {
  final Customer customer;

  const CustomerDetailScreen({super.key, required this.customer});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final currentCustomer = useState<Customer>(customer);
    final updatingType = useState(false);
    final loadError = useState<Object?>(null);
    final currentTags = useState<List<CustomerTag>>(List.from(customer.tags));
    final defaults = ref.read(defaultErrorTextProvider);

    Future<void> loadDetail() async {
      loadError.value = null;
      try {
        final detail = await ref
            .read(customerRepositoryProvider)
            .getCustomer(customer.id);
        if (!context.mounted) return;
        if (detail != null) {
          currentCustomer.value = detail;
          currentTags.value = detail.tags;
        }
      } catch (e) {
        if (context.mounted) loadError.value = e;
      }
    }

    useEffect(() {
      loadDetail();
      return null;
    }, []);

    final c = currentCustomer.value;
    final isVip = c.isVip;

    Future<List<CustomerTag>?> addTag(String name) async {
      try {
        final updated = await ref
            .read(customerRepositoryProvider)
            .addCustomerTags(customer.id, [name]);
        currentTags.value = updated.tags;
        return updated.tags;
      } catch (e) {
        if (context.mounted) {
          FloatingToast.error(context, errorText(e, defaults: defaults));
        }
        return null;
      }
    }

    Future<List<CustomerTag>?> removeTag(CustomerTag tag) async {
      try {
        final updated = await ref
            .read(customerRepositoryProvider)
            .removeCustomerTag(customer.id, tag.id);
        currentTags.value = updated.tags;
        return updated.tags;
      } catch (e) {
        if (context.mounted) {
          FloatingToast.error(context, errorText(e, defaults: defaults));
        }
        return null;
      }
    }

    void popWithResult() {
      context.pop(CustomerDetailResult(customerId: customer.id, isVip: isVip));
    }

    void openOrders() {
      context.push(
        '/customer/orders',
        extra: {
          'customerName': c.displayName(context.uiLocale, context.l10n),
          'customerId': customer.id,
        },
      );
    }

    Future<void> toggleVip() async {
      if (updatingType.value) return;
      final next = isVip
          ? CustomerType.CUSTOMER_TYPE_NORMAL
          : CustomerType.CUSTOMER_TYPE_VIP;
      updatingType.value = true;
      try {
        final updated = await ref
            .read(customerRepositoryProvider)
            .updateCustomerType(customer.id, next);
        if (!context.mounted) return;
        currentCustomer.value = currentCustomer.value.copyWith(
          type: updated.type,
        );
        FloatingToast.success(context, isVip ? '已取消 VIP' : '已升級為 VIP');
      } catch (e) {
        if (context.mounted) {
          FloatingToast.error(context, errorText(e, defaults: defaults));
        }
      } finally {
        if (context.mounted) updatingType.value = false;
      }
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
        ],
      );
      if (value == 'vip' && context.mounted) await toggleVip();
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
          title: Text(c.displayName(context.uiLocale, context.l10n)),
          subtitle: '顧客 360',
          onBackPressed: popWithResult,
          actions: [
            IconButton(
              tooltip: '聯絡顧客',
              onPressed: () => context.push(
                '/chat/thread',
                extra: {
                  'title': c.displayName(context.uiLocale, context.l10n),
                  'participantId': c.id,
                },
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
              if (loadError.value != null)
                Padding(
                  key: const ValueKey('customer-detail-load-error'),
                  padding: EdgeInsets.only(bottom: 12.h),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          errorText(loadError.value!, defaults: defaults),
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.danger,
                          ),
                        ),
                      ),
                      TextButton(
                        onPressed: loadDetail,
                        child: HkButtonLabel(context.l10n.commonRetry),
                      ),
                    ],
                  ),
                ),
              _buildProfileSection(
                c,
                isVip,
                currentTags.value,
                context,
                onAddTag: addTag,
                onRemoveTag: removeTag,
              ),
              SizedBox(height: 16.h),
              Container(height: 1, color: const Color(0x1A42493F)),
              SizedBox(height: 16.h),
              _buildStatsSection(c, openOrders),
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
    List<CustomerTag> tags,
    BuildContext context, {
    required Future<List<CustomerTag>?> Function(String name) onAddTag,
    required Future<List<CustomerTag>?> Function(CustomerTag tag) onRemoveTag,
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
          ],
        ),
        SizedBox(height: 12.h),
        Text(
          customer.displayName(context.uiLocale, context.l10n),
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
    required Future<List<CustomerTag>?> Function(String name) onAddTag,
    required Future<List<CustomerTag>?> Function(CustomerTag tag) onRemoveTag,
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

  Widget _buildStatsSection(Customer customer, VoidCallback onOpenOrders) {
    return Row(
      children: [
        Expanded(
          child: _buildStatCard(
            icon: AppIcons.monetizationOnOutlined,
            label: '消費總金額',
            value: customer.totalSpent.toString(),
            onTap: onOpenOrders,
          ),
        ),
        SizedBox(width: 16.w),
        Expanded(
          child: _buildStatCard(
            icon: AppIcons.assignmentOutlined,
            label: '訂單數',
            value: customer.orderCount.toString(),
            onTap: onOpenOrders,
          ),
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

  void _showTagManagementSheet(
    BuildContext context,
    List<CustomerTag> currentTags,
    Future<List<CustomerTag>?> Function(String) onAddTag,
    Future<List<CustomerTag>?> Function(CustomerTag) onRemoveTag,
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
  final Future<List<CustomerTag>?> Function(String) onAddTag;
  final Future<List<CustomerTag>?> Function(CustomerTag) onRemoveTag;

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
      final updated = await onAddTag(trimmed);
      if (updated != null) {
        tags.value = updated;
        hasChanges.value = true;
        textCtrl.clear();
      }
      if (context.mounted) isLoading.value = false;
    }

    Future<void> removeTag(CustomerTag tag) async {
      isLoading.value = true;
      final updated = await onRemoveTag(tag);
      if (updated != null) {
        tags.value = updated;
        hasChanges.value = true;
      }
      if (context.mounted) isLoading.value = false;
    }

    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
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
                CustomerTagManagementHeader(
                  onClose: () => Navigator.pop(context),
                ),
                SizedBox(height: 20.h),

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
