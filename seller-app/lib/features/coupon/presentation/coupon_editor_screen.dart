import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:path/path.dart' as p;

import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../customer/data/customer_model.dart';
import '../../customer/data/customer_repository.dart';
import '../data/coupon_model.dart';
import '../data/coupon_repository.dart';
import 'coupon_controller.dart';
import 'widgets/date_picker_card.dart';
import 'widgets/form_card.dart';
import 'widgets/preview_card.dart';
import 'widgets/specify_user_card.dart';
import 'widgets/submit_button.dart';
import 'widgets/tag_selection_card.dart';
import 'widgets/target_user_card.dart';
import 'widgets/top_bar.dart';

/// 優惠券完整設定頁面（進階版）
///
/// 入口：優惠券列表頁「新增優惠券」按鈕，或未來的編輯入口。
///
/// ## 功能
/// - 支援三種折扣類型：現金折抵 / 折數折抵 / 整單折抵
/// - 圖片上傳（圓形封面）
/// - 完整日期選擇：快選天數 / 無期限 / 日曆月視圖
/// - 最高折抵上限（百分比折扣時顯示）
/// - 目標用戶群：全部會員 / 消費過會員 / 指定會員 / TAG 標籤
/// - 提交成功後顯示 [_CouponSentDialog] 彈窗
///
/// ## 與 AddCouponScreen 的差異
/// - 無 [PreselectedUser] 預填參數（不從 CRM 帶入用戶）
/// - 功能更完整，適合在優惠券管理頁自行建立
///
/// [coupon] 為 null → 建立模式，否則為編輯模式（保留欄位）。
class CouponEditorScreen extends HookConsumerWidget {
  const CouponEditorScreen({super.key, this.coupon});
  final Coupon? coupon;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // ── Scroll ──
    final scrollCtrl = useScrollController();

    // ── Form controllers ──
    final nameCtrl = useTextEditingController(text: coupon?.title ?? '');
    final discountCtrl = useTextEditingController(
      text: _initialDiscountText(coupon),
    );
    final minPurchaseCtrl = useTextEditingController(
      text: coupon?.thresholdAmount ?? '',
    );
    final maxDiscountCtrl = useTextEditingController(
      text: coupon?.maxDiscountAmount ?? '',
    );

    // ── Local state via hooks ──
    final discountRule = useState(_initialDiscountRule(coupon));
    final saving = useState(false);

    // Date picker state
    final noExpiry = useState(_initialNoExpiry(coupon));
    final selectedDays = useState<int?>(null);
    final useCustomDate = useState(false);
    final validUntil = useState(_initialValidUntil(coupon));
    final hasExpiry = useState(
      coupon?.endAt != null && coupon!.endAt!.isNotEmpty,
    );
    final calendarMonth = useState(DateTime.now());

    // Target user state
    final targetUserType = useState('全部會員');
    final phoneSearchCtrl = useTextEditingController();
    final selectedUsers = useState<List<Map<String, String>>>([]);
    final isLookingUp = useState(false);
    final lookupError = useState<String?>(null);

    // TAG 標籤狀態
    final allTags = useState<List<CustomerTag>>([]);
    final selectedTagIds = useState<Set<int>>({});
    final tagCustomers = useState<List<Customer>>([]);
    final tagLoading = useState(false);

    // Photo state
    final couponPhotoBytes = useState<Uint8List?>(null);
    final couponPhotoFilename = useState<String?>(null);
    final couponPhotoUrl = useState<String?>(null);
    final photoChanged = useState(false);

    // Force rebuild on text changes
    final formVersion = useState(0);
    useEffect(() {
      void onChange() => formVersion.value++;
      nameCtrl.addListener(onChange);
      discountCtrl.addListener(onChange);
      minPurchaseCtrl.addListener(onChange);
      maxDiscountCtrl.addListener(onChange);
      return () {
        nameCtrl.removeListener(onChange);
        discountCtrl.removeListener(onChange);
        minPurchaseCtrl.removeListener(onChange);
        maxDiscountCtrl.removeListener(onChange);
      };
    }, [nameCtrl, discountCtrl, minPurchaseCtrl, maxDiscountCtrl]);

    // ── Helpers (closures) ──
    String fmtDate(DateTime d) =>
        '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

    bool isFormComplete() {
      if (nameCtrl.text.trim().isEmpty) return false;
      if (discountCtrl.text.trim().isEmpty) return false;
      if (!hasExpiry.value && !noExpiry.value) return false;
      return true;
    }

    Future<void> onPickPhoto() async {
      try {
        final source = await showModalBottomSheet<ImageSource>(
          context: context,
          backgroundColor: AppColors.bgPrimary,
          builder: (ctx) => SafeArea(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  leading: const Icon(AppIcons.photoLibraryOutlined),
                  title: const Text('從相簿選擇'),
                  onTap: () => Navigator.of(ctx).pop(ImageSource.gallery),
                ),
                ListTile(
                  leading: const Icon(AppIcons.photoCameraOutlined),
                  title: const Text('拍攝照片'),
                  onTap: () => Navigator.of(ctx).pop(ImageSource.camera),
                ),
                SizedBox(height: 8.h),
              ],
            ),
          ),
        );
        if (source == null) return;
        final picker = ImagePicker();
        final XFile? file = await picker.pickImage(
          source: source,
          maxWidth: 1280.w,
          maxHeight: 1280.h,
          imageQuality: 85,
        );
        if (file == null) return;
        final bytes = await file.readAsBytes();
        if (!context.mounted) return;
        final name = file.name.isNotEmpty ? file.name : p.basename(file.path);
        couponPhotoBytes.value = bytes;
        couponPhotoFilename.value = name;
        photoChanged.value = true;
      } on PlatformException {
        if (!context.mounted) return;
        FloatingToast.error(context, '無法開啟相機或相簿');
      } catch (_) {
        if (!context.mounted) return;
        FloatingToast.error(context, '選取圖片失敗，請重試');
      }
    }

    Future<void> onSave() async {
      if (saving.value) return;
      final title = nameCtrl.text.trim();
      if (title.isEmpty) {
        FloatingToast.error(context, '請輸入優惠券名稱');
        return;
      }
      saving.value = true;
      try {
        // Map UI discount rule to API type code
        final String type = discountRule.value == '折數折抵'
            ? CouponType.percent.value
            : CouponType.cash.value;
        // Build request body matching CreateCouponTemplateRequest
        // targetType: 1=ALL 全體買家, 2=MEMBER 賣家會員, 3=DESIGNATED 指定買家, 4=COMPENSATION 補償券
        final String targetType;
        if (targetUserType.value == '消費過會員') {
          targetType = '2';
        } else if (targetUserType.value == '指定會員' ||
            targetUserType.value == 'TAG 標籤') {
          targetType = '3';
        } else {
          targetType = '1';
        }
        final now = DateTime.now();
        final Map<String, dynamic> body = {
          'title': title,
          'type': type,
          'targetType': targetType,
          'startAt': fmtDate(now),
        };

        // End date
        if (!noExpiry.value) {
          body['endAt'] = fmtDate(validUntil.value);
        } else {
          // 無期限：設一年後
          body['endAt'] = fmtDate(DateTime(now.year + 1, now.month, now.day));
        }

        // Threshold (minimum purchase)
        final min = minPurchaseCtrl.text.trim();
        if (min.isNotEmpty) body['thresholdAmount'] = min;

        // Discount value
        final disc = discountCtrl.text.trim();
        if (disc.isNotEmpty) {
          if (type == CouponType.percent.value) {
            body['discountPercent'] = disc;
          } else {
            body['discountAmount'] = disc;
          }
        }
        if (type == CouponType.percent.value) {
          final maxDisc = maxDiscountCtrl.text.trim();
          if (maxDisc.isNotEmpty) body['maxDiscountAmount'] = maxDisc;
        }

        // targetType=3 (DESIGNATED) 時附帶 buyerIds——TAG 顧客與手動查詢結果
        // 皆為 CRM/lookup 已解析好的現任 buyer_id，不是 account_id，故送
        // buyerIds（gateway 只驗證格式、不重複解析），不是 buyerAccountIds。
        if (targetType == '3') {
          if (targetUserType.value == 'TAG 標籤') {
            body['buyerIds'] = tagCustomers.value
                .map((c) => c.id)
                .where((id) => id.isNotEmpty && id != '0')
                .toList();
          } else {
            body['buyerIds'] = selectedUsers.value
                .map((u) => u['accountId'] ?? u['id'] ?? '')
                .where((id) => id.isNotEmpty)
                .toList();
          }
        }

        final notifier = ref.read(couponListControllerProvider.notifier);
        await notifier.create(body);
        if (!context.mounted) return;
        await showDialog<void>(
          context: context,
          barrierColor: AppColors.textEmphasis.withValues(alpha: 0.35),
          builder: (_) => const _CouponSentDialog(),
        );
        if (!context.mounted) return;
        context.pop('created');
      } catch (e) {
        if (!context.mounted) return;
        final msg = e is AppException ? e.message : e.toString();
        FloatingToast.error(context, msg);
      } finally {
        saving.value = false;
      }
    }

    Future<void> onAddUser() async {
      final searchText = phoneSearchCtrl.text.trim();
      if (searchText.isEmpty) {
        FloatingToast.error(context, '請輸入手機號碼');
        return;
      }
      if (isLookingUp.value) return;

      // Check for duplicates
      final alreadyAdded = selectedUsers.value.any(
        (u) => u['accountId'] == searchText || u['phone'] == searchText,
      );
      if (alreadyAdded) {
        FloatingToast.error(context, '該用戶已添加');
        return;
      }

      isLookingUp.value = true;
      lookupError.value = null;
      try {
        final repo = ref.read(couponRepositoryProvider);
        final result = await repo.lookupByPhone(searchText);

        if (!context.mounted) return;

        // 用 uid 做去重
        final alreadyExists = selectedUsers.value.any(
          (u) => u['accountId'] == result.uid,
        );
        if (alreadyExists) {
          FloatingToast.error(context, '該用戶已添加');
          return;
        }

        selectedUsers.value = [
          ...selectedUsers.value,
          {
            'name': result.buyerName.isNotEmpty ? result.buyerName : '用戶',
            'accountId': result.uid,
            'phone': result.phone,
            'avatarUrl': result.avatarUrl ?? '',
          },
        ];
        phoneSearchCtrl.clear();
        lookupError.value = null;
      } on AppException catch (e) {
        lookupError.value = e.message;
      } catch (_) {
        lookupError.value = '查詢失敗，請稍後再試';
      } finally {
        isLookingUp.value = false;
      }
    }

    // Touch formVersion to stay reactive
    formVersion.value;

    // ── Build ──

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.dark,
      child: Scaffold(
        backgroundColor: AppColors.surfacePage,
        body: Column(
          children: [
            CouponTopBar(title: '優惠券設定', onBack: () => context.pop()),

            // ── Content ──
            Expanded(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [
                      AppColors.primarySoft,
                      AppColors.surfacePage,
                      AppColors.brandSecondarySoft.withValues(alpha: 0.18),
                    ],
                    stops: const [0, 0.46, 1],
                  ),
                ),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final gutter = HkBreakpoints.pageGutter(
                      constraints.maxWidth,
                    );
                    return Align(
                      alignment: Alignment.topCenter,
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(
                          maxWidth: HkControl.chromeMaxWidth,
                        ),
                        child: ListView(
                          controller: scrollCtrl,
                          padding: EdgeInsets.fromLTRB(
                            gutter,
                            HkSpace.lg,
                            gutter,
                            HkSpace.large,
                          ),
                          children: [
                            // Preview card
                            CouponPreviewCard(
                              nameCtrl: nameCtrl,
                              discountCtrl: discountCtrl,
                              minPurchaseCtrl: minPurchaseCtrl,
                              discountRule: discountRule.value,
                              noExpiry: noExpiry.value,
                              hasExpiry: hasExpiry.value,
                              validUntil: validUntil.value,
                              photoBytes: couponPhotoBytes.value,
                              photoUrl: couponPhotoUrl.value,
                              onPickPhoto: saving.value ? null : onPickPhoto,
                            ),
                            const SizedBox(height: HkSpace.lg),

                            // Form card
                            CouponFormCard(
                              nameCtrl: nameCtrl,
                              discountCtrl: discountCtrl,
                              minPurchaseCtrl: minPurchaseCtrl,
                              discountRule: discountRule.value,
                              onDiscountRuleChanged: (v) =>
                                  discountRule.value = v,
                              maxDiscountAmountCtrl: maxDiscountCtrl,
                            ),
                            const SizedBox(height: HkSpace.lg),

                            // Date picker card
                            CouponDatePickerCard(
                              noExpiry: noExpiry.value,
                              selectedDays: selectedDays.value,
                              useCustomDate: useCustomDate.value,
                              validUntil: validUntil.value,
                              hasExpiry: hasExpiry.value,
                              calendarMonth: calendarMonth.value,
                              onNoExpiryChanged: (v) {
                                noExpiry.value = v;
                                if (v) {
                                  useCustomDate.value = false;
                                  selectedDays.value = null;
                                  hasExpiry.value = false;
                                }
                              },
                              onCustomDateChanged: (v) {
                                useCustomDate.value = v;
                                if (v) {
                                  selectedDays.value = null;
                                  noExpiry.value = false;
                                }
                              },
                              onDaysSelected: (days) {
                                selectedDays.value = days;
                                useCustomDate.value = false;
                                validUntil.value = DateTime.now().add(
                                  Duration(days: days),
                                );
                                hasExpiry.value = true;
                                noExpiry.value = false;
                              },
                              onDateSelected: (date) {
                                validUntil.value = date;
                                hasExpiry.value = true;
                                noExpiry.value = false;
                                useCustomDate.value = true;
                                selectedDays.value = null;
                              },
                              onCalendarMonthChanged: (m) =>
                                  calendarMonth.value = m,
                            ),
                            const SizedBox(height: HkSpace.lg),

                            // Target user card
                            CouponTargetUserCard(
                              targetUserType: targetUserType.value,
                              onTypeSelected: (v) => targetUserType.value = v,
                            ),
                            const SizedBox(height: HkSpace.lg),

                            // Specify user card（僅指定會員時顯示）
                            if (targetUserType.value == '指定會員') ...[
                              CouponSpecifyUserCard(
                                phoneSearchCtrl: phoneSearchCtrl,
                                selectedUsers: selectedUsers.value,
                                isLoading: isLookingUp.value,
                                errorMessage: lookupError.value,
                                onAddUser: onAddUser,
                                onRemoveUser: (index) {
                                  final list = [...selectedUsers.value];
                                  list.removeAt(index);
                                  selectedUsers.value = list;
                                },
                              ),
                              const SizedBox(height: HkSpace.lg),
                            ],

                            // TAG 標籤選擇（僅 TAG 標籤時顯示）
                            if (targetUserType.value == 'TAG 標籤') ...[
                              CouponTagSelectionCard(
                                allTags: allTags.value,
                                selectedTagIds: selectedTagIds.value,
                                matchedCustomers: tagCustomers.value,
                                isLoading: tagLoading.value,
                                onLoadTags: () async {
                                  if (allTags.value.isNotEmpty) return;
                                  tagLoading.value = true;
                                  try {
                                    final tags = await ref
                                        .read(customerRepositoryProvider)
                                        .getSellerTags();
                                    allTags.value = tags;
                                  } catch (_) {}
                                  tagLoading.value = false;
                                },
                                onTagToggled: (tagId) async {
                                  final newSet = Set<int>.from(
                                    selectedTagIds.value,
                                  );
                                  newSet.contains(tagId)
                                      ? newSet.remove(tagId)
                                      : newSet.add(tagId);
                                  selectedTagIds.value = newSet;
                                  if (newSet.isEmpty) {
                                    tagCustomers.value = [];
                                    return;
                                  }
                                  tagLoading.value = true;
                                  try {
                                    tagCustomers.value = await ref
                                        .read(customerRepositoryProvider)
                                        .getCustomers(
                                          tagId: newSet.first,
                                          pageSize: 100,
                                        );
                                  } catch (_) {
                                    tagCustomers.value = [];
                                  }
                                  tagLoading.value = false;
                                },
                                onRemoveCustomer: (idx) {
                                  final list = [...tagCustomers.value];
                                  list.removeAt(idx);
                                  tagCustomers.value = list;
                                },
                              ),
                              const SizedBox(height: HkSpace.lg),
                            ],

                            // Submit button
                            CouponSubmitButton(
                              isComplete: isFormComplete(),
                              saving: saving.value,
                              onTap: onSave,
                            ),
                            const SizedBox(height: HkSpace.xxl),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ── Static init helpers ──

  static String _initialDiscountRule(Coupon? c) {
    if (c == null) return '現金折抵';
    return CodeMetaDisplay.couponTypeMeta(c.type) == CouponType.percent
        ? '折數折抵'
        : '現金折抵';
  }

  static String _initialDiscountText(Coupon? c) {
    if (c == null) return '';
    return CodeMetaDisplay.couponTypeMeta(c.type) == CouponType.percent
        ? (c.discountPercent ?? '')
        : (c.discountAmount ?? '');
  }

  static bool _initialNoExpiry(Coupon? c) {
    if (c == null) return false;
    return c.endAt == null || c.endAt!.isEmpty;
  }

  static DateTime _initialValidUntil(Coupon? c) {
    if (c == null) return DateTime.now();
    if (c.endAt != null && c.endAt!.isNotEmpty) {
      final parsed = DateTime.tryParse(c.endAt!);
      if (parsed != null) return parsed;
    }
    return DateTime.now();
  }
}

/// 發放成功提示彈窗
class _CouponSentDialog extends StatelessWidget {
  const _CouponSentDialog();

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(HkSpace.section),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 360),
        child: HkFrostedCardSurface(
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.xxl),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 56,
                  height: 56,
                  decoration: const BoxDecoration(
                    color: AppColors.successSoft,
                    shape: BoxShape.circle,
                  ),
                  child: const Center(
                    child: HkIcon(
                      HkIconName.check,
                      size: 30,
                      color: AppIconColors.success,
                    ),
                  ),
                ),
                const SizedBox(height: HkSpace.lg),
                const Text(
                  '優惠券已建立',
                  style: TextStyle(
                    color: AppColors.textEmphasis,
                    fontSize: 20,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: HkSpace.sm),
                const Text(
                  '已依你選擇的對象完成發放。',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 14,
                    height: 1.5,
                  ),
                ),
                const SizedBox(height: HkSpace.xxl),
                SizedBox(
                  width: double.infinity,
                  height: 48,
                  child: FilledButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const HkButtonLabel('完成'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
