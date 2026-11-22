import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/utils/hk_log.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_dropdown.dart';
import '../../customer/data/customer_model.dart';
import '../../customer/data/customer_repository.dart';
import 'coupon_controller.dart';
import 'widgets/customer_picker_sheet.dart';
import 'widgets/section_card.dart';
import 'widgets/selected_users_list.dart';
import 'tag_audience_toggle.dart';
import 'widgets/tag_selection_card.dart';
import 'widgets/top_bar.dart';

/// 預設選擇的用戶資料（從客戶 CRM 傳入）
class PreselectedUser {
  final String name;
  final String id;

  const PreselectedUser({required this.name, required this.id});
}

/// 快速發放優惠券頁面（簡化版）
///
/// 入口：顧客消費記錄頁（顧客詳情）的「發優惠券」按鈕，
/// 可帶入 [PreselectedUser] 預填指定顧客，省去手動搜尋步驟。
///
/// ## 功能
/// - 支援兩種折扣類型：固定金額 / 百分比折扣
/// - 有效期快選（7 / 15 / 30 / 60 天）或指定日期
/// - 目標用戶群：全部會員 / 消費過會員 / 指定會員 / TAG 標籤
/// - 提交成功後顯示 Toast 並 pop 回上一頁
///
/// ## 與 CouponEditorScreen 的差異
/// - 接受 [PreselectedUser] 預填用戶（從 CRM 帶入）
/// - 無圖片上傳、無最高折抵上限、無無期限選項
/// - 流程更簡短，適合從顧客管理快速發放
///
/// [preselectedUser] 為從 CRM 畫面帶入的指定用戶，可為 null。
class AddCouponScreen extends HookConsumerWidget {
  const AddCouponScreen({super.key, this.preselectedUser});
  final PreselectedUser? preselectedUser;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // ── Form controllers ──
    final nameCtrl = useTextEditingController();
    final discountCtrl = useTextEditingController();
    final thresholdCtrl = useTextEditingController();

    // ── Local state ──
    final discountType = useState(CouponType.percent.code);
    final selectedDays = useState<int?>(30);
    final useCustomDate = useState(false);
    final selectedDate = useState(DateTime.now().add(const Duration(days: 30)));
    final selectedTargetGroup = useState('指定會員');
    final saving = useState(false);

    // TAG 標籤相關狀態
    final allTags = useState<List<CustomerTag>>([]);
    final selectedTagIds = useState<Set<String>>({});
    final tagCustomers = useState<List<Customer>>([]);
    final tagLoading = useState(false);
    final tagToggle = useMemoized(
      () => TagAudienceToggle<Customer>(
        fetch: (tagId) =>
            ref.read(customerRepositoryProvider).listAllCustomersWithTag(tagId),
        setSelected: (v) => selectedTagIds.value = v,
        setCustomers: (v) => tagCustomers.value = v,
        setLoading: (v) => tagLoading.value = v,
        onError: (e) {
          if (!context.mounted) return;
          FloatingToast.error(
            context,
            errorText(e, defaults: ref.read(defaultErrorTextProvider)),
          );
        },
      ),
    );

    // Selected users
    final selectedUsers = useState<List<Map<String, String>>>(() {
      if (preselectedUser != null) {
        return [
          {'name': preselectedUser!.name, 'id': preselectedUser!.id},
        ];
      }
      return <Map<String, String>>[];
    }());

    // Force rebuild on text changes
    final formVersion = useState(0);
    useEffect(() {
      void onChange() => formVersion.value++;
      nameCtrl.addListener(onChange);
      discountCtrl.addListener(onChange);
      thresholdCtrl.addListener(onChange);
      return () {
        nameCtrl.removeListener(onChange);
        discountCtrl.removeListener(onChange);
        thresholdCtrl.removeListener(onChange);
      };
    }, [nameCtrl, discountCtrl, thresholdCtrl]);
    // Touch formVersion to stay reactive
    formVersion.value;

    // ── Helpers ──
    String fmtDate(DateTime d) =>
        '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

    bool isFormComplete() {
      if (nameCtrl.text.trim().isEmpty) return false;
      if (discountCtrl.text.trim().isEmpty) return false;
      if (selectedDays.value == null && !useCustomDate.value) return false;
      if (selectedTargetGroup.value == '指定會員' && selectedUsers.value.isEmpty) {
        return false;
      }
      if (selectedTargetGroup.value == 'TAG 標籤' &&
          (selectedTagIds.value.isEmpty || tagCustomers.value.isEmpty)) {
        return false;
      }
      return true;
    }

    Future<void> onSubmit() async {
      if (saving.value || !isFormComplete()) return;
      saving.value = true;
      try {
        final type = discountType.value;
        // targetType: 1=ALL 全體買家, 2=MEMBER 賣家會員, 3=DESIGNATED 指定買家, 4=COMPENSATION 補償券
        final String targetType;
        if (selectedTargetGroup.value == '消費過會員') {
          targetType = '2';
        } else if (selectedTargetGroup.value == '指定會員' ||
            selectedTargetGroup.value == 'TAG 標籤') {
          targetType = '3';
        } else {
          targetType = '1';
        }
        final Map<String, dynamic> body = {
          'title': nameCtrl.text.trim(),
          'type': type,
          'targetType': targetType,
          'startAt': fmtDate(DateTime.now()),
        };

        // Expiry
        body['endAt'] = fmtDate(selectedDate.value);

        // Discount value
        final disc = discountCtrl.text.trim();
        if (disc.isNotEmpty) {
          if (type == CouponType.percent.code) {
            body['discountPercent'] = disc;
          } else {
            body['discountAmount'] = disc;
          }
        }

        // Threshold
        final min = thresholdCtrl.text.trim();
        if (min.isNotEmpty) body['thresholdAmount'] = min;

        // targetType=3 (DESIGNATED) 時附帶 buyerIds——TAG 顧客與手動查詢結果
        // 皆為 CRM/lookup 已解析好的現任 buyer_id，不是 account_id，故送
        // buyerIds（gateway 只驗證格式、不重複解析），不是 buyerAccountIds。
        if (targetType == '3') {
          if (selectedTargetGroup.value == 'TAG 標籤') {
            body['buyerIds'] = tagCustomers.value
                .map((c) => c.id)
                .where((id) => id.isNotEmpty && id != '0')
                .toList();
          } else {
            body['buyerIds'] = selectedUsers.value
                .map((u) => u['id'] ?? '')
                .where((id) => id.isNotEmpty)
                .toList();
          }
        }

        // Create coupon (new API creates + issues in one call)
        final notifier = ref.read(couponListControllerProvider.notifier);
        HKLog.logE('[Coupon] create coupon body: $body');
        await notifier.create(body);
        if (!context.mounted) return;

        if (selectedUsers.value.isEmpty) {
          FloatingToast.success(context, '已建立優惠券');
        } else {
          FloatingToast.success(
            context,
            '已發送給 ${selectedUsers.value.length} 位顧客',
          );
        }
        context.pop('created');
      } catch (e) {
        if (!context.mounted) return;
        FloatingToast.error(context, '發送失敗，請稍後再試');
      } finally {
        saving.value = false;
      }
    }

    Future<void> onPickUsers() async {
      final picked = await showCouponCustomerPicker(
        context,
        excludedIds: {for (final u in selectedUsers.value) u['id'] ?? ''},
      );
      if (picked == null || picked.isEmpty || !context.mounted) return;
      selectedUsers.value = [
        ...selectedUsers.value,
        for (final c in picked) {'name': c.displayName, 'id': c.id},
      ];
    }

    // ── Build ──
    const themeColor = AppColors.primary;
    const bgColor = AppColors.bgPrimary;
    final bottomPad = MediaQuery.of(context).padding.bottom;

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.dark,
      child: Scaffold(
        backgroundColor: bgColor,
        body: Column(
          children: [
            CouponTopBar(title: '新增優惠券', onBack: () => context.pop()),

            // Content
            Expanded(
              child: SingleChildScrollView(
                padding: EdgeInsets.all(16.w),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Preview card
                    _PreviewCard(
                      nameCtrl: nameCtrl,
                      discountCtrl: discountCtrl,
                      thresholdCtrl: thresholdCtrl,
                      discountType: discountType.value,
                      selectedDays: selectedDays.value,
                      useCustomDate: useCustomDate.value,
                      selectedDate: selectedDate.value,
                    ),
                    SizedBox(height: 16.h),

                    // Form card
                    _AddCouponFormCard(
                      nameCtrl: nameCtrl,
                      discountCtrl: discountCtrl,
                      thresholdCtrl: thresholdCtrl,
                      discountType: discountType.value,
                      onDiscountTypeChanged: (v) => discountType.value = v,
                    ),
                    SizedBox(height: 16.h),

                    // Validity card
                    _ValidityCard(
                      selectedDays: selectedDays.value,
                      useCustomDate: useCustomDate.value,
                      selectedDate: selectedDate.value,
                      onDaysSelected: (days) {
                        selectedDays.value = days;
                        useCustomDate.value = false;
                        selectedDate.value = DateTime.now().add(
                          Duration(days: days),
                        );
                      },
                      onCustomDateChanged: (v) {
                        useCustomDate.value = v;
                        if (v) selectedDays.value = null;
                      },
                      onDateSelected: (d) {
                        selectedDate.value = d;
                        useCustomDate.value = true;
                        selectedDays.value = null;
                      },
                    ),
                    SizedBox(height: 16.h),

                    // Target group card
                    _TargetGroupCard(
                      selectedGroup: selectedTargetGroup.value,
                      onGroupChanged: (v) => selectedTargetGroup.value = v,
                    ),
                    SizedBox(height: 16.h),

                    // Specify user / tags section
                    if (selectedTargetGroup.value == '指定會員')
                      _SpecifyUserSection(
                        selectedUsers: selectedUsers.value,
                        onPickUsers: onPickUsers,
                        onRemoveUser: (i) {
                          final list = [...selectedUsers.value];
                          list.removeAt(i);
                          selectedUsers.value = list;
                        },
                      ),
                    if (selectedTargetGroup.value == 'TAG 標籤')
                      CouponTagSelectionCard(
                        allTags: allTags.value,
                        selectedTagIds: selectedTagIds.value,
                        matchedCustomers: tagCustomers.value,
                        isLoading: tagLoading.value,
                        onLoadTags: () async {
                          if (allTags.value.isNotEmpty) return;
                          tagLoading.value = true;
                          try {
                            allTags.value = await ref
                                .read(customerRepositoryProvider)
                                .listTags();
                          } catch (e) {
                            if (context.mounted) {
                              FloatingToast.error(
                                context,
                                errorText(
                                  e,
                                  defaults: ref.read(defaultErrorTextProvider),
                                ),
                              );
                            }
                          }
                          tagLoading.value = false;
                        },
                        onTagToggled: (tagId) =>
                            tagToggle.toggle(tagId, selectedTagIds.value),
                        onRemoveCustomer: (idx) {
                          final list = [...tagCustomers.value];
                          list.removeAt(idx);
                          tagCustomers.value = list;
                        },
                      ),
                    SizedBox(height: 24.h),
                  ],
                ),
              ),
            ),

            // Bottom button
            Container(
              padding: EdgeInsets.only(
                left: 16.w,
                right: 16.w,
                top: 12.h,
                bottom: bottomPad + 12,
              ),
              color: bgColor,
              child: SizedBox(
                width: double.infinity,
                height: 48.h,
                child: ElevatedButton(
                  onPressed: (saving.value || !isFormComplete())
                      ? null
                      : onSubmit,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: themeColor,
                    disabledBackgroundColor: AppColors.gray300,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                  ),
                  child: saving.value
                      ? SizedBox(
                          width: 24.w,
                          height: 24.h,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: AppColors.bgCard,
                          ),
                        )
                      : HkButtonLabel(
                          '發放優惠券',
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.bgCard,
                          ),
                        ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// Sub-widgets
// ═══════════════════════════════════════════════════════════════════

class _PreviewCard extends StatelessWidget {
  const _PreviewCard({
    required this.nameCtrl,
    required this.discountCtrl,
    required this.thresholdCtrl,
    required this.discountType,
    required this.selectedDays,
    required this.useCustomDate,
    required this.selectedDate,
  });

  final TextEditingController nameCtrl;
  final TextEditingController discountCtrl;
  final TextEditingController thresholdCtrl;
  final String discountType;
  final int? selectedDays;
  final bool useCustomDate;
  final DateTime selectedDate;

  @override
  Widget build(BuildContext context) {
    const textColor = AppColors.textPrimary;
    final name = nameCtrl.text.isEmpty ? '優惠卷名稱' : nameCtrl.text;
    final isPercentage = discountType == CouponType.percent.code;
    final discountValue = discountCtrl.text.isEmpty ? '0' : discountCtrl.text;
    final threshold = thresholdCtrl.text.isEmpty ? '0' : thresholdCtrl.text;

    String validityText;
    if (selectedDays != null) {
      validityText = '$selectedDays天';
    } else if (useCustomDate) {
      validityText =
          '${selectedDate.year}/${selectedDate.month.toString().padLeft(2, '0')}/${selectedDate.day.toString().padLeft(2, '0')}';
    } else {
      validityText = '--';
    }

    return CouponSectionCard(
      padding: EdgeInsets.all(20.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            name,
            style: TextStyle(
              fontWeight: FontWeight.w700,
              fontSize: 20.sp,
              color: textColor,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          SizedBox(height: 12.h),
          Text(
            isPercentage ? '折數：$discountValue折' : '現金折抵：NT\$$discountValue',
            style: TextStyle(color: textColor, fontSize: 16.sp),
          ),
          if (threshold != '0') ...[
            SizedBox(height: 8.h),
            Text(
              '・門檻：NT\$$threshold',
              style: TextStyle(color: textColor, fontSize: 16.sp),
            ),
          ],
          SizedBox(height: 8.h),
          Row(
            children: [
              Icon(
                AppIcons.accessTimeOutlined,
                size: 18.sp,
                color: AppIconColors.muted,
              ),
              SizedBox(width: 6.w),
              Text(
                '有效期限：$validityText',
                style: TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 16.sp,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _AddCouponFormCard extends StatelessWidget {
  const _AddCouponFormCard({
    required this.nameCtrl,
    required this.discountCtrl,
    required this.thresholdCtrl,
    required this.discountType,
    required this.onDiscountTypeChanged,
  });

  final TextEditingController nameCtrl;
  final TextEditingController discountCtrl;
  final TextEditingController thresholdCtrl;
  final String discountType;
  final ValueChanged<String> onDiscountTypeChanged;

  @override
  Widget build(BuildContext context) {
    const textColor = AppColors.textPrimary;

    const inputHeight = 40.0;

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '優惠卷名稱',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 8.h),
          SizedBox(
            height: inputHeight,
            child: TextField(
              controller: nameCtrl,
              style: TextStyle(color: textColor, fontSize: 14.sp),
              decoration: _inputDecor('填寫優惠卷名稱'),
            ),
          ),
          SizedBox(height: 20.h),

          Text(
            '折扣方式',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 8.h),
          Row(
            children: [
              _RuleBtn(
                label: '固定金額',
                icon: '\$',
                isSelected: discountType == CouponType.cash.code,
                onTap: () => onDiscountTypeChanged(CouponType.cash.code),
              ),
              SizedBox(width: 12.w),
              _RuleBtn(
                label: '百分比折扣',
                icon: '%',
                isSelected: discountType == CouponType.percent.code,
                onTap: () => onDiscountTypeChanged(CouponType.percent.code),
              ),
            ],
          ),
          SizedBox(height: 20.h),

          Text(
            discountType == CouponType.percent.code ? '折扣百分比' : '折扣金額',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 8.h),
          SizedBox(
            height: inputHeight,
            child: TextField(
              controller: discountCtrl,
              keyboardType: const TextInputType.numberWithOptions(
                decimal: true,
              ),
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[0-9\.]')),
              ],
              style: TextStyle(color: textColor, fontSize: 14.sp),
              decoration: _inputDecor(
                discountType == CouponType.percent.code ? '折' : 'NT\$',
              ),
            ),
          ),
          SizedBox(height: 20.h),

          Text(
            '最低消費門檻',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 8.h),
          SizedBox(
            height: inputHeight,
            child: TextField(
              controller: thresholdCtrl,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              style: TextStyle(color: textColor, fontSize: 14.sp),
              decoration: _inputDecor('NT\$'),
            ),
          ),
        ],
      ),
    );
  }

  static InputDecoration _inputDecor(String hint) {
    return InputDecoration(
      hintText: hint,
      hintStyle: TextStyle(color: AppColors.textSecondary, fontSize: 14.sp),
      filled: true,
      fillColor: AppColors.bgCard,
      contentPadding: EdgeInsets.symmetric(horizontal: 12.w),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12.r),
        borderSide: const BorderSide(color: AppColors.cardBorder),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12.r),
        borderSide: const BorderSide(color: AppColors.cardBorder),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12.r),
        borderSide: const BorderSide(color: AppColors.primary),
      ),
    );
  }
}

class _RuleBtn extends StatelessWidget {
  const _RuleBtn({
    required this.label,
    required this.icon,
    required this.isSelected,
    required this.onTap,
  });
  final String label;
  final String icon;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    const primaryGreen = AppColors.primary;
    const textColor = AppColors.textPrimary;
    return Expanded(
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          height: 40.h,
          decoration: BoxDecoration(
            color: isSelected ? primaryGreen : AppColors.bgCard,
            borderRadius: BorderRadius.circular(12.r),
            border: Border.all(
              color: isSelected ? primaryGreen : AppColors.cardBorder,
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                icon,
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: isSelected ? AppColors.bgCard : textColor,
                ),
              ),
              SizedBox(width: 4.w),
              Text(
                label,
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: isSelected ? FontWeight.w700 : FontWeight.normal,
                  color: isSelected ? AppColors.bgCard : textColor,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ValidityCard extends StatelessWidget {
  const _ValidityCard({
    required this.selectedDays,
    required this.useCustomDate,
    required this.selectedDate,
    required this.onDaysSelected,
    required this.onCustomDateChanged,
    required this.onDateSelected,
  });

  final int? selectedDays;
  final bool useCustomDate;
  final DateTime selectedDate;
  final ValueChanged<int> onDaysSelected;
  final ValueChanged<bool> onCustomDateChanged;
  final ValueChanged<DateTime> onDateSelected;

  @override
  Widget build(BuildContext context) {
    const textColor = AppColors.textPrimary;
    const themeColor = AppColors.primary;

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '有效期限',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 12.h),
          Row(
            children: [7, 15, 30, 60].map((days) {
              final isSelected = selectedDays == days;
              return Expanded(
                child: Padding(
                  padding: EdgeInsets.only(right: days == 60 ? 0 : 8),
                  child: GestureDetector(
                    onTap: () => onDaysSelected(days),
                    child: Container(
                      height: 36.h,
                      decoration: BoxDecoration(
                        color: isSelected ? themeColor : AppColors.bgCard,
                        borderRadius: BorderRadius.circular(12.r),
                        border: Border.all(
                          color: isSelected ? themeColor : AppColors.cardBorder,
                        ),
                      ),
                      child: Center(
                        child: Text(
                          '$days天',
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: isSelected ? AppColors.bgCard : textColor,
                            fontWeight: isSelected
                                ? FontWeight.w700
                                : FontWeight.normal,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              );
            }).toList(),
          ),
          SizedBox(height: 12.h),
          GestureDetector(
            onTap: () => onCustomDateChanged(!useCustomDate),
            child: Row(
              children: [
                Container(
                  width: 20.w,
                  height: 20.h,
                  decoration: BoxDecoration(
                    color: useCustomDate ? themeColor : AppColors.bgCard,
                    borderRadius: BorderRadius.circular(6.r),
                    border: Border.all(
                      color: useCustomDate
                          ? themeColor
                          : AppColors.textSecondary,
                    ),
                  ),
                  child: useCustomDate
                      ? Icon(
                          AppIcons.check,
                          size: 14.sp,
                          color: AppIconColors.onBrand,
                        )
                      : null,
                ),
                SizedBox(width: 8.w),
                Text(
                  useCustomDate
                      ? '指定日期: ${selectedDate.year}/${selectedDate.month.toString().padLeft(2, '0')}/${selectedDate.day.toString().padLeft(2, '0')}'
                      : '指定日期',
                  style: TextStyle(fontSize: 14.sp, color: textColor),
                ),
              ],
            ),
          ),
          if (useCustomDate) ...[
            SizedBox(height: 8.h),
            CalendarDatePicker(
              initialDate: selectedDate,
              firstDate: DateTime.now(),
              lastDate: DateTime.now().add(const Duration(days: 730)),
              onDateChanged: onDateSelected,
            ),
          ],
        ],
      ),
    );
  }
}

class _TargetGroupCard extends StatelessWidget {
  const _TargetGroupCard({
    required this.selectedGroup,
    required this.onGroupChanged,
  });

  final String selectedGroup;
  final ValueChanged<String> onGroupChanged;

  @override
  Widget build(BuildContext context) {
    const textColor = AppColors.textPrimary;
    final groups = ['全部會員', '消費過會員', '指定會員', 'TAG 標籤'];

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '目標用戶群',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 12.h),
          AppDropdown(
            items: groups,
            selectedItem: selectedGroup,
            onSelected: onGroupChanged,
            showSearch: false,
            child: Container(
              height: 44.h,
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                border: Border.all(color: AppColors.cardBorder),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      selectedGroup,
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: textColor,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),
                  Icon(
                    AppIcons.keyboardArrowDown,
                    size: 18.sp,
                    color: AppIconColors.defaultColor,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SpecifyUserSection extends StatelessWidget {
  const _SpecifyUserSection({
    required this.selectedUsers,
    required this.onPickUsers,
    required this.onRemoveUser,
  });

  final List<Map<String, String>> selectedUsers;
  final VoidCallback onPickUsers;
  final ValueChanged<int> onRemoveUser;

  @override
  Widget build(BuildContext context) {
    const textColor = AppColors.textPrimary;
    const themeColor = AppColors.primary;

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '指定用戶',
            style: TextStyle(
              fontSize: 18.sp,
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          SizedBox(height: 12.h),
          GestureDetector(
            key: const ValueKey('coupon-pick-customers'),
            onTap: onPickUsers,
            child: Container(
              height: 40.h,
              padding: EdgeInsets.symmetric(horizontal: 12.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                border: Border.all(color: AppColors.cardBorder),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    AppIcons.personAddOutlined,
                    size: 18.sp,
                    color: themeColor,
                  ),
                  SizedBox(width: 4.w),
                  Text(
                    '從客戶名單選擇',
                    style: TextStyle(fontSize: 14.sp, color: themeColor),
                  ),
                ],
              ),
            ),
          ),
          CouponSelectedUsersList(users: selectedUsers, onRemove: onRemoveUser),
        ],
      ),
    );
  }
}
