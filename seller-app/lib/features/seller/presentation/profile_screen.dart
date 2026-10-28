import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/utils/app_version_utils.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../core/router/tab_refresh_provider.dart';
import '../../../core/session/my_store.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/platform_io.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_page_section.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../media/data/media_repository.dart';
import '../../notification/data/notification_repository.dart';
import '../../post/presentation/widgets/page_header.dart';
import '../data/current_store.dart';
import '../data/current_kyc.dart';
import '../data/onboarding_tasks.dart';
import '../data/seller_status_refresh.dart';
import '../data/store_editor.dart';
import '../data/store_form.dart';
import '../data/version_conflict.dart';
import 'store_address_edit_dialog.dart';
import 'profile_avatar_section.dart' show ProfileAvatarCircle;
import 'onboarding_progress_card.dart';
import 'profile_edit_dialog.dart';
import 'profile_form_widgets.dart';
import 'profile_account_closure.dart';
import 'profile_other_options_card.dart';
import 'seller_certification_gate.dart';

const _readOnlyNotice = '商店目前無法編輯，這裡只能檢視';
const _savedNotice = '商店資訊已儲存';

/// 商家設定（商店資料）頁面。
///
/// - 封面圖 / 頭像：上傳取得 staged token，再以 `UpdateMyStore` 換上
/// - 商店資料：點選欄位編輯，每次只寫那個欄位（update_mask），帶讀到的 etag
/// - 其他設定入口（物流、通知、幫助）、登出
///
/// 資料來自 `GetMyStore`（`myStoreSnapshotProvider`）；寫入成功或版本衝突後重讀。
class ProfileScreen extends HookConsumerWidget {
  const ProfileScreen({super.key, this.showSellerSection = false});

  /// 是否為 tab 內的設定頁（返回回到儀表板，不 pop）。
  final bool showSellerSection;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final mine = ref.watch(myStoreSnapshotProvider);
    final store = mine?.store;
    final canWrite =
        mine?.can(SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE) ?? false;
    final canClose =
        mine?.can(SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE) ?? false;
    final kyc = ref.watch(currentKycProvider)?.value;
    final payuniLocked = kyc?.identityLocked ?? false;
    final shippingGate = mine?.shippingGate ?? ShippingGate.unknown;
    final profileInfoKey = useMemoized(GlobalKey.new);

    final isUploadingAvatar = useState(false);
    final unreadNotifCount = useState<int>(0);

    Future<void> loadUnreadCount() async {
      try {
        final result = await ref
            .read(notificationRepositoryProvider)
            .getUnreadCount();
        if (context.mounted) unreadNotifCount.value = result.count;
      } catch (_) {}
    }

    void refreshProgress() {
      ref.invalidate(onboardingInputsProvider);
      unawaited(ref.read(sellerStatusRefreshProvider.notifier).refresh());
    }

    useEffect(() {
      loadUnreadCount();
      refreshProgress();
      return null;
    }, const []);

    ref.listen(settingsTabRefreshProvider, (prev, next) {
      if (prev != next) {
        loadUnreadCount();
        refreshProgress();
      }
    });

    void goToProfileInfo() {
      final target = profileInfoKey.currentContext;
      if (target == null) return;
      unawaited(
        Scrollable.ensureVisible(
          target,
          duration: const Duration(milliseconds: 300),
        ),
      );
    }

    void handleBackPressed() {
      if (!context.mounted) return;
      if (!showSellerSection && context.canPop()) {
        context.pop();
        return;
      }
      context.go('/main/dashboard');
    }

    Future<void> editText(StoreTextField field) async {
      var wrote = false;
      await showProfileEditDialog(
        context,
        title: field.label,
        initialValue: field.valueOf(ref.read(myStoreSnapshotProvider)?.store),
        keyboardType: switch (field) {
          StoreTextField.contactPhone => TextInputType.phone,
          StoreTextField.contactMobile => TextInputType.number,
          StoreTextField.supportEmail => TextInputType.emailAddress,
          StoreTextField.websiteUrl => TextInputType.url,
          _ => TextInputType.text,
        },
        maxLines: field.maxLines,
        maxLength: field.maxLength,
        placeholder: field.placeholder,
        hint: field.hint,
        inputFormatters: field == StoreTextField.contactMobile
            ? [FilteringTextInputFormatter.digitsOnly]
            : const [],
        onSave: (raw) async {
          final value = field.normalize(raw);
          final invalid = field.validate(value);
          if (invalid != null) return invalid;
          final current = field.valueOf(
            ref.read(myStoreSnapshotProvider)?.store,
          );
          if (value == current) return null;
          final outcome = await ref
              .read(storeEditorProvider)
              .save(field.changes(value));
          final message = storeSaveMessage(
            outcome,
            fieldPaths: [field.errorPath],
          );
          wrote = message == null;
          return message;
        },
      );
      if (wrote && context.mounted) {
        FloatingToast.success(context, _savedNotice);
      }
    }

    Future<void> editAddress() async {
      final address = ref.read(myStoreSnapshotProvider)?.store?.address;
      var wrote = false;
      await showDialog<bool>(
        context: context,
        barrierDismissible: true,
        builder: (_) => StoreAddressEditDialog(
          initialTownCode: address?.townCode ?? '',
          initialDetail: address?.addressDetail ?? '',
          onSave: (townCode, detail) async {
            if (townCode == (address?.townCode ?? '') &&
                detail == (address?.addressDetail ?? '')) {
              return null;
            }
            final changes = townCode.isEmpty
                ? const StoreChanges(clearAddress: true)
                : const StoreChanges().withAddress(
                    townCode: townCode,
                    addressDetail: detail,
                  );
            final outcome = await ref.read(storeEditorProvider).save(changes);
            final message = storeSaveMessage(
              outcome,
              fieldPaths: ['store.address'],
            );
            wrote = message == null;
            return message;
          },
        ),
      );
      if (wrote && context.mounted) {
        FloatingToast.success(context, _savedNotice);
      }
    }

    Future<void> saveImage({
      required StoreChanges changes,
      required String successMessage,
      required String failureMessage,
    }) async {
      final outcome = await ref.read(storeEditorProvider).save(changes);
      if (!context.mounted) return;
      switch (outcome) {
        case StoreSaved():
          FloatingToast.success(context, successMessage);
        case StoreSaveConflict():
          FloatingToast.error(
            context,
            storeSaveMessage(
                  outcome,
                  conflictGuidance: versionConflictImageGuidance,
                ) ??
                failureMessage,
          );
        default:
          FloatingToast.error(
            context,
            storeSaveMessage(outcome) ?? failureMessage,
          );
      }
    }

    /// 從相冊選取頭像，確認後上傳並換上。
    Future<void> pickImage() async {
      try {
        final picker = ImagePicker();
        final XFile? image = await picker.pickImage(
          source: ImageSource.gallery,
          maxWidth: 1024.w,
          maxHeight: 1024.h,
          imageQuality: 85,
        );
        if (image == null) return;
        if (!context.mounted) return;

        // 顯示確認對話框（附預覽）
        final confirm = await showDialog<bool>(
          context: context,
          barrierDismissible: false,
          builder: (ctx) {
            return Dialog(
              backgroundColor: Colors.transparent,
              insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
              child: Container(
                decoration: BoxDecoration(
                  color: AppColors.dividerLight,
                  borderRadius: BorderRadius.circular(24.r),
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: double.infinity,
                      padding: EdgeInsets.symmetric(
                        vertical: 24.h,
                        horizontal: 16.w,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.vertical(
                          top: Radius.circular(24.r),
                        ),
                      ),
                      child: Column(
                        children: [
                          Text(
                            '使用此照片作為頭像？',
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              fontSize: 18.sp,
                              color: AppColors.textEmphasis,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          SizedBox(height: 20.h),
                          ProfileAvatarCircle(
                            size: 140,
                            child: Image.file(
                              File(image.path),
                              fit: BoxFit.cover,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const Divider(
                      height: 0,
                      thickness: 1,
                      color: AppColors.gray300,
                    ),
                    Row(
                      children: [
                        Expanded(
                          child: Material(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.only(
                              bottomLeft: Radius.circular(24.r),
                            ),
                            child: InkWell(
                              borderRadius: BorderRadius.only(
                                bottomLeft: Radius.circular(24.r),
                              ),
                              onTap: () => Navigator.of(ctx).pop(false),
                              child: SizedBox(
                                height: 54.h,
                                child: Center(
                                  child: Text(
                                    '否',
                                    style: TextStyle(
                                      fontSize: 20.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.textEmphasis,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ),
                        Container(
                          width: 1,
                          height: 54.h,
                          color: AppColors.gray300,
                        ),
                        Expanded(
                          child: Material(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.only(
                              bottomRight: Radius.circular(24.r),
                            ),
                            child: InkWell(
                              borderRadius: BorderRadius.only(
                                bottomRight: Radius.circular(24.r),
                              ),
                              onTap: () => Navigator.of(ctx).pop(true),
                              child: SizedBox(
                                height: 54.h,
                                child: Center(
                                  child: Text(
                                    '是',
                                    style: TextStyle(
                                      fontSize: 20.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.textEmphasis,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            );
          },
        );
        if (confirm != true) return;

        isUploadingAvatar.value = true;
        final bytes = await image.readAsBytes();
        final result = await ref
            .read(mediaRepositoryProvider)
            .uploadAvatar(bytes: bytes);
        final token = result.stagedToken;
        if (token == null || token.isEmpty) {
          if (context.mounted) FloatingToast.error(context, '上傳失敗，請稍後再試');
          return;
        }
        await saveImage(
          changes: StoreChanges(logoStagedToken: token),
          successMessage: '頭像上傳成功',
          failureMessage: '上傳頭像失敗，請稍後再試',
        );
      } catch (e, stack) {
        HKLog.logE('[Profile] Avatar upload exception: $e\n$stack');
        if (context.mounted) FloatingToast.error(context, '上傳頭像失敗，請稍後再試');
      } finally {
        if (context.mounted) isUploadingAvatar.value = false;
      }
    }

    /// 從相冊選取封面圖，上傳並換上。
    Future<void> pickSellerCover() async {
      final picker = ImagePicker();
      final XFile? picked = await picker.pickImage(
        source: ImageSource.gallery,
        maxWidth: 1920,
        imageQuality: 90,
      );
      if (picked == null) return;
      final bytes = await picked.readAsBytes();
      try {
        final result = await ref
            .read(mediaRepositoryProvider)
            .uploadWithSession(
              bytes: bytes,
              filename: 'cover_${DateTime.now().millisecondsSinceEpoch}.jpg',
              compress: true,
            );
        final token = result.stagedToken;
        if (token == null || token.isEmpty) {
          if (context.mounted) FloatingToast.error(context, '背景圖片上傳失敗');
          return;
        }
        await saveImage(
          changes: StoreChanges(coverStagedToken: token),
          successMessage: '背景圖片上傳成功',
          failureMessage: '背景圖片上傳失敗',
        );
      } catch (e, stack) {
        HKLog.logE('[Profile] Cover upload error: $e\n$stack');
        if (context.mounted) FloatingToast.error(context, '背景圖片上傳失敗');
      }
    }

    String valueOf(StoreTextField field) => field.valueOf(store);

    ProfileEditableInfoRow textRow(
      StoreTextField field, {
      bool locked = false,
    }) => ProfileEditableInfoRow(
      label: field.label,
      value: valueOf(field),
      placeholder: field.placeholder,
      maxLines: field.maxLines > 1 ? field.maxLines.clamp(1, 4) : 1,
      lockedNotice: locked && payuniLocked ? kPayuniLockedNotice : null,
      onTap: canWrite ? () => editText(field) : null,
    );

    final address = mine?.addressText ?? '';
    final location = mine?.locationText ?? '';
    final fansCount = mine?.followerCount;
    final topInset = MediaQuery.viewPaddingOf(context).top;
    final sellerName = mine?.storeName ?? '尚未設定店舖名稱';
    final prize = valueOf(StoreTextField.prize);
    final description = valueOf(StoreTextField.description);
    final layoutSpec = PageHeaderLayoutSpec.of(context);
    final headerHeight =
        topInset +
        layoutSpec.expandedProfileHeightFor(
          hasAward: prize.isNotEmpty,
          awardText: prize,
          fansCount: fansCount,
          displayedLocation: location.isEmpty ? '尚未設定店舖位置' : location,
          description: description,
        );
    final overlayStyle = SystemUiOverlayStyle.dark.copyWith(
      statusBarColor: Colors.transparent,
      systemNavigationBarColor: AppColors.bgPrimary,
      systemNavigationBarIconBrightness: Brightness.dark,
    );

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: overlayStyle,
      child: PopScope(
        canPop: !showSellerSection,
        onPopInvokedWithResult: (didPop, _) {
          if (didPop) return;
          handleBackPressed();
        },
        child: Scaffold(
          backgroundColor: AppColors.bgPrimary,
          body: store == null
              ? const TruckLoadingOverlay()
              : CustomScrollView(
                  key: const PageStorageKey<String>('seller-settings-scroll'),
                  slivers: [
                    SliverToBoxAdapter(
                      child: SizedBox(
                        height: headerHeight,
                        child: PageHeader(
                          coverUrl: mine?.coverUrl,
                          sellerName: sellerName,
                          fansCount: fansCount,
                          ratingAverage: mine?.ratingAverage,
                          reviewCount: mine?.reviewCount,
                          location: location,
                          prize: prize,
                          description: description,
                          avatarUrl: mine?.logoUrl,
                          title: '設定',
                          onBackPressed: !showSellerSection
                              ? handleBackPressed
                              : null,
                          showNotifications: true,
                          onNotifications: () => context.push('/notifications'),
                          onChat: () => context.push('/chat'),
                          onPickCover: canWrite ? pickSellerCover : null,
                          onPickAvatar: canWrite ? pickImage : null,
                          isUploadingAvatar: isUploadingAvatar.value,
                        ),
                      ),
                    ),
                    SliverToBoxAdapter(
                      child: HkAdaptivePage(
                        child: Padding(
                          padding: const EdgeInsets.only(
                            top: HkSpace.lg,
                            bottom: 120,
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              KeyedSubtree(
                                key: profileInfoKey,
                                child: ProfileInfoSection(
                                  title: '商家資訊',
                                  description: canWrite
                                      ? '店舖名稱、所在地與商家介紹會同步到買家店舖頁。'
                                      : _readOnlyNotice,
                                  rows: [
                                    textRow(
                                      StoreTextField.storeName,
                                      locked: true,
                                    ),
                                    textRow(
                                      StoreTextField.description,
                                      locked: true,
                                    ),
                                    ProfileAddressRow(
                                      value: address,
                                      onTap: canWrite ? editAddress : null,
                                    ),
                                    textRow(StoreTextField.websiteUrl),
                                    textRow(StoreTextField.prize),
                                  ],
                                ),
                              ),
                              const SizedBox(height: HkSpace.section),
                              ProfileInfoSection(
                                title: '聯絡資訊',
                                description: '用於平台聯繫與店舖營運通知。',
                                rows: [
                                  textRow(StoreTextField.contactPhone),
                                  textRow(StoreTextField.contactName),
                                  textRow(StoreTextField.contactMobile),
                                  textRow(StoreTextField.supportEmail),
                                ],
                              ),
                              const SizedBox(height: HkSpace.section),
                              OnboardingProgressCard(
                                onOpenProfile: goToProfileInfo,
                              ),
                              const SizedBox(height: HkSpace.section),
                              HkPageSection(
                                title: '營運設定',
                                child: ProfileOtherOptionsCard(
                                  shippingGate: shippingGate,
                                  onShippingTap: () =>
                                      openShippingSettings(context, ref),
                                  notifUnread: unreadNotifCount.value,
                                ),
                              ),
                              const SizedBox(height: HkSpace.section),
                              const HkPageSection(
                                title: '帳號',
                                description: '登出不會刪除資料。',
                                child: HkContentCard(
                                  child: ProfileLogoutButton(),
                                ),
                              ),
                              if (canClose) ...[
                                const SizedBox(height: HkSpace.section),
                                const ProfileCloseStoreSection(),
                              ],
                              const SizedBox(height: HkSpace.section),
                              const ProfileDeleteAccountSection(),
                              const SizedBox(height: HkSpace.xxl),
                              Center(
                                child: FutureBuilder<String>(
                                  future: fetchAppVersion(),
                                  builder: (context, snapshot) {
                                    final v = snapshot.data?.isNotEmpty == true
                                        ? 'v.${snapshot.data}'
                                        : 'v.-';
                                    return Text(
                                      'Hawkr 賣家 $v',
                                      style: const TextStyle(
                                        color: AppColors.textSecondary,
                                        fontSize: 12,
                                        letterSpacing: 1.0,
                                      ),
                                    );
                                  },
                                ),
                              ),
                              const SizedBox(height: HkControl.navHeight),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
        ),
      ),
    );
  }
}
