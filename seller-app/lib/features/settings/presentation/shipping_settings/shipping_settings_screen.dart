import '../../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/utils/hk_log.dart';
import 'package:hawkr_seller/shared/utils/floating_toast.dart';
import 'package:hawkr_seller/shared/utils/text_length_validator.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../data/cvs_shipping_mutation.dart';
import '../../data/shipping_sender.dart';
import '../shipping_settings_controller.dart';
import '../../../../shared/widgets/app_confirm_dialog.dart';
import '../../../../shared/widgets/hk_content_card.dart';
import '../../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/hk_page_section.dart';
import '../store_map_webview_screen.dart';
import 'widgets/common_widgets.dart';
import 'widgets/cvs_shipping_section.dart';
import 'widgets/taqbin_shipping_section.dart';

/// 物流與退貨設定頁面 (重構後版本)
/// 使用模組化組件結構，提高代碼可維護性
class ShippingSettingsScreen extends HookConsumerWidget {
  const ShippingSettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final senderRows =
        useState<({Map<String, dynamic>? home, Map<String, dynamic>? cvs})?>(
          null,
        );
    final saving = useState(false);

    final taqbinExpanded = useState(true);
    final taqbinSelected = useState(false);
    final taqbinLocked = useState(false);
    final taqbinPayuniStatus = useState<String>('');
    final taqbinProviderMessage = useState<String>('');
    final homeNameCtrl = useTextEditingController();
    final homePhoneCtrl = useTextEditingController();
    final homeCity = useState<String?>(null);
    final homeDistrict = useState<String?>(null);
    final homeAddressCtrl = useTextEditingController();

    final cvsExpanded = useState(true);
    final cvsSelected = useState(false);
    final cvsEnabled = useState(false);
    final cvsSenderLocked = useState(false);
    final cvsProcessing = useState(false);
    final cvsNameCtrl = useTextEditingController();
    final cvsPhoneCtrl = useTextEditingController();
    final selectedStoreName = useState<String?>(null);
    final selectedStoreCode = useState<String?>(null);
    final cvsInitialSnapshot = useState<String>('');
    final cvsOnboardingId = useState<String?>(null);
    final cvsInitialEnabled = useState(false);
    final shippingController = ref.read(
      shippingSettingsControllerProvider.notifier,
    );
    final cvsMutationCoordinator = useMemoized(
      () => CvsShippingMutationCoordinator(shippingController.sendCvsMutation),
      const [],
    );

    String normalizeText(String? value) => (value ?? '').trim();

    String buildCvsSnapshot() {
      return [
        cvsSelected.value ? '1' : '0',
        normalizeText(cvsNameCtrl.text),
        normalizeText(cvsPhoneCtrl.text),
        normalizeText(selectedStoreCode.value),
        normalizeText(selectedStoreName.value),
      ].join('|');
    }

    Future<void> loadData() async {
      loading.value = true;
      try {
        final bootstrap = await shippingController.loadBootstrap();
        HKLog.logD(
          '[ShippingSettings] onboardings loaded: '
          '${bootstrap.onboardings.length}',
        );
        Map<String, dynamic>? homeRow;
        Map<String, dynamic>? cvsRow;
        var cvsToggleable = false;
        for (final o in bootstrap.onboardings) {
          final type = o['shippingType']?.toString() ?? '';
          HKLog.logD(
            '[ShippingSettings] record: shippingType=$type '
            'payuniSubmitStatus=${o['payuniSubmitStatus']} '
            'onboardingStatus=${o['onboardingStatus']} senderName=${o['senderName']}',
          );
          final onboardingType =
              ShippingType.fromValue(type) ?? ShippingType.fromCode(type);
          if (onboardingType == ShippingType.homeDelivery) {
            homeRow ??= o;
          } else if (onboardingType == ShippingType.cvsC2c) {
            final actualEnabled = o['enabled']?.toString() == 'true';
            final onboardingId =
                o['onboardingId']?.toString() ??
                o['onboarding_id']?.toString() ??
                o['id']?.toString() ??
                '';
            cvsRow ??= o;
            if (onboardingId.isNotEmpty) {
              cvsOnboardingId.value = onboardingId;
            }
            cvsInitialEnabled.value = actualEnabled;
            cvsSelected.value = actualEnabled;
            if (isShippingRowToggleable(o)) cvsToggleable = true;
          } else {
            HKLog.logW(
              '[ShippingSettings] 未支援的 onboarding shippingType=$type '
              '（本版僅支援 3651 CVS_C2C / 3652 HOME_DELIVERY），該筆設定不會顯示',
            );
          }
        }

        if (homeRow != null) {
          final payuniStatus = homeRow['payuniSubmitStatus']?.toString() ?? '';
          if (payuniStatus.isNotEmpty) taqbinPayuniStatus.value = payuniStatus;
          if (homeRow['providerMessage'] != null) {
            taqbinProviderMessage.value = homeRow['providerMessage'].toString();
          }
          if (isShippingRowLocked(homeRow)) {
            taqbinSelected.value = true;
            taqbinLocked.value = true;
          }
        }
        final cvsReference = cvsRow;
        if (cvsReference != null) {
          cvsEnabled.value = cvsToggleable;
          cvsSenderLocked.value = isShippingRowLocked(cvsReference);
          cvsProcessing.value =
              cvsReference['payuniSubmitStatus']?.toString() ==
              payuniSubmitRequested;
          final storeCode = cvsReference['returnStoreCode']?.toString() ?? '';
          final storeName = cvsReference['returnStoreName']?.toString() ?? '';
          if (storeCode.isNotEmpty) selectedStoreCode.value = storeCode;
          if (storeName.isNotEmpty) selectedStoreName.value = storeName;
        }
        senderRows.value = (home: homeRow, cvs: cvsReference);
      } catch (_) {}
      senderRows.value ??= (home: null, cvs: null);
      cvsInitialSnapshot.value = buildCvsSnapshot();
      loading.value = false;
    }

    final store = ref.watch(myStoreSnapshotProvider);
    final homeApplied = useRef(false);
    final cvsApplied = useRef(false);
    useEffect(() {
      final rows = senderRows.value;
      if (rows == null) return null;
      final cvsClean = cvsInitialSnapshot.value == buildCvsSnapshot();
      if (!homeApplied.value) {
        final home = homeSender(rows.home, store);
        if (!home.isEmpty) {
          homeApplied.value = true;
          if (homeNameCtrl.text.isEmpty) homeNameCtrl.text = home.name;
          if (homePhoneCtrl.text.isEmpty) homePhoneCtrl.text = home.phone;
          if (homeCity.value == null &&
              homeDistrict.value == null &&
              homeAddressCtrl.text.isEmpty) {
            homeCity.value = home.city.isEmpty ? null : home.city;
            homeDistrict.value = home.district.isEmpty ? null : home.district;
            homeAddressCtrl.text = home.addressDetail;
          }
        }
      }
      if (!cvsApplied.value) {
        final cvs = cvsSender(rows.cvs, store);
        if (!cvs.isEmpty) {
          cvsApplied.value = true;
          if (cvsNameCtrl.text.isEmpty) cvsNameCtrl.text = cvs.name;
          if (cvsPhoneCtrl.text.isEmpty) cvsPhoneCtrl.text = cvs.phone;
          if (cvsClean) cvsInitialSnapshot.value = buildCvsSnapshot();
        }
      }
      return null;
    }, [senderRows.value, store]);

    useEffect(() {
      loadData();
      return null;
    }, const []);

    Future<void> pickStore() async {
      try {
        final resp = await shippingController.loadStoreMapSession();
        if (resp == null) {
          if (context.mounted) FloatingToast.error(context, '無法取得門市地圖');
          return;
        }
        final url = resp['redirectUrl']?.toString() ?? '';
        if (url.isEmpty) {
          if (context.mounted) FloatingToast.error(context, '無法取得門市地圖連結');
          return;
        }
        if (!context.mounted) return;
        final result = await Navigator.of(context).push<Map<String, String>>(
          MaterialPageRoute(
            builder: (_) => StoreMapWebViewScreen(redirectUrl: url),
          ),
        );
        if (!context.mounted) return;
        if (result != null) {
          final error = result['error'] ?? result['message'] ?? '';
          if (error.isNotEmpty) {
            FloatingToast.error(context, error);
            return;
          }
          final storeId = result['storeId'] ?? result['StoreID'] ?? '';
          final storeName = result['storeName'] ?? result['StoreName'] ?? '';
          if (storeId.isEmpty) {
            FloatingToast.error(context, '未取得門市資料，請重新選擇');
            return;
          }
          if (storeId.isNotEmpty) selectedStoreCode.value = storeId;
          if (storeName.isNotEmpty) selectedStoreName.value = storeName;
        }
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '開啟門市地圖失敗');
      }
    }

    final hasTaqbin = taqbinSelected.value;
    final hasCvs = cvsSelected.value;
    final cvsDirty = cvsInitialSnapshot.value != buildCvsSnapshot();
    void toggleTaqbinExpanded() {
      taqbinExpanded.value = !taqbinExpanded.value;
    }

    void toggleCvsExpanded() {
      cvsExpanded.value = !cvsExpanded.value;
    }

    Future<void> onActivateTaqbin() async {
      final confirmed = await showAppConfirmDialog(
        context,
        title: '確認開通黑貓物流？',
        message: '開通後，黑貓的收貨 / 退貨地址將不可在 App 內修改。',
        cancelText: '返回修改',
        confirmText: '確認開通',
      );
      if (!context.mounted) return;
      if (!confirmed) return;

      final taqbinNameError = TextLengthValidator.validateName(
        homeNameCtrl.text,
        maxDisplayLength: taqbinSenderNameMaxDisplayLength,
        fieldLabel: '聯絡人姓名',
      );
      if (taqbinNameError != null) {
        FloatingToast.error(context, taqbinNameError);
        return;
      }
      if (homePhoneCtrl.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入手機號碼');
        return;
      }
      if (homeCity.value == null) {
        FloatingToast.error(context, '請選擇縣市');
        return;
      }
      if (homeDistrict.value == null) {
        FloatingToast.error(context, '請選擇區域');
        return;
      }
      if (homeAddressCtrl.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入詳細地址');
        return;
      }

      final postalCode = postalCodeOfSelection(
        homeCity.value,
        homeDistrict.value,
      );
      if (postalCode == null) {
        FloatingToast.error(context, '請重新選擇行政區');
        return;
      }

      saving.value = true;

      try {
        final result = await shippingController.activateHomeDelivery({
          'productCategory': '0001',
          'senderName': homeNameCtrl.text.trim(),
          'senderPhone': homePhoneCtrl.text.trim(),
          'senderCity': homeCity.value ?? '',
          'senderDistrict': homeDistrict.value ?? '',
          'senderAddressDetail': homeAddressCtrl.text.trim(),
          'senderPostalCode': postalCode,
        });

        if (!result.isSuccess) {
          if (context.mounted) {
            FloatingToast.error(context, '開通失敗：${result.message}');
          }
        } else if (context.mounted) {
          taqbinLocked.value = true;
          FloatingToast.success(context, '黑貓物流已開通');
          context.pop();
        }
      } catch (e) {
        if (context.mounted) FloatingToast.error(context, '開通失敗：$e');
      }
      saving.value = false;
    }

    Future<void> onActivateCvs() async {
      if (saving.value || cvsMutationCoordinator.isRunning) return;

      if (selectedStoreCode.value == null || selectedStoreCode.value!.isEmpty) {
        FloatingToast.error(context, '請選擇退貨 7-11 門市');
        return;
      }

      final cvsNameTrimmed = cvsNameCtrl.text.trim();
      if (!cvsEnabled.value || cvsNameTrimmed.isNotEmpty) {
        final cvsNameError = TextLengthValidator.validateName(
          cvsNameCtrl.text,
          maxDisplayLength: cvsSenderNameMaxDisplayLength,
          fieldLabel: '退貨人姓名',
        );
        if (cvsNameError != null) {
          FloatingToast.error(context, cvsNameError);
          return;
        }
      }

      saving.value = true;
      try {
        final cvsBody = {
          'senderName': cvsNameCtrl.text.trim(),
          'senderPhone': cvsPhoneCtrl.text.trim(),
          'returnStoreCode': selectedStoreCode.value ?? '',
          if (selectedStoreName.value != null)
            'returnStoreName': selectedStoreName.value,
        };

        final outcome = await cvsMutationCoordinator.apply([
          CvsGoodsTypeMutation(
            initialEnabled: cvsInitialEnabled.value,
            targetEnabled: cvsSelected.value,
            onboardingId: cvsOnboardingId.value,
            createBody: cvsBody,
          ),
        ]);

        if (!context.mounted) return;
        for (final applied in outcome.completed) {
          cvsInitialEnabled.value = applied.mutation.targetEnabled;
          if (applied.onboardingId case final id?) {
            cvsOnboardingId.value = id;
          }
        }
        if (outcome.completed.any((item) => item.mutation.targetEnabled)) {
          cvsEnabled.value = true;
          cvsSenderLocked.value = true;
        }

        if (outcome.isSuccess) {
          cvsInitialSnapshot.value = buildCvsSnapshot();
          FloatingToast.success(context, '7-11物流已更新');
          context.pop();
          return;
        }

        FloatingToast.error(
          context,
          '${outcome.failedMutation?.label ?? '物流'}更新失敗：'
          '${outcome.error?.message ?? '請稍後再試'}。請重試',
        );
      } finally {
        if (context.mounted) saving.value = false;
      }
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: const HkGlassAppBar(
        title: Text('物流與退貨設定'),
        subtitle: '黑貓宅急便與 7-11 店到店',
      ),
      body: loading.value
          ? const TruckLoadingOverlay(message: '載入中...')
          : DecoratedBox(
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
                  final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
                  return Align(
                    alignment: Alignment.topCenter,
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(
                        maxWidth: HkControl.chromeMaxWidth,
                      ),
                      child: SingleChildScrollView(
                        padding: EdgeInsets.fromLTRB(
                          gutter,
                          HkSpace.lg,
                          gutter,
                          HkSpace.large,
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const HkPageSection(
                              title: '配送方式',
                              description: '開通前請確認聯絡人與退貨資料。',
                            ),
                            const SizedBox(height: HkSpace.lg),

                            TaqbinShippingSection(
                              expanded: taqbinExpanded,
                              onToggleExpanded: toggleTaqbinExpanded,
                              enabled: taqbinLocked.value,
                              selected: taqbinSelected,
                              nameCtrl: homeNameCtrl,
                              phoneCtrl: homePhoneCtrl,
                              city: homeCity,
                              district: homeDistrict,
                              addressCtrl: homeAddressCtrl,
                              onCitySelected: (val) {
                                homeCity.value = val;
                                homeDistrict.value = null;
                              },
                              onDistrictSelected: (val) {
                                homeDistrict.value = val;
                              },
                            ),
                            const SizedBox(height: HkSpace.md),

                            if (taqbinExpanded.value) ...[
                              HkContentCard(
                                padding: const EdgeInsets.all(HkSpace.md),
                                backgroundColor: AppColors.surfaceMuted,
                                child: Row(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    const HkIcon(
                                      HkIconName.info,
                                      size: 20,
                                      color: AppIconColors.active,
                                    ),
                                    const SizedBox(width: HkSpace.sm),
                                    Expanded(
                                      child: Text(
                                        '黑貓地址開通後由 PayUni 管理。如需修改，請聯繫 02-6605-0810 或 service@example.com。',
                                        style: AppTypography.metaMuted,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const SizedBox(height: HkSpace.md),

                              if (taqbinPayuniStatus.value ==
                                  payuniSubmitRequested) ...[
                                const SaveButton(
                                  canSave: false,
                                  saving: true,
                                  buttonText: '處理中…',
                                ),
                                SizedBox(height: 16.h),
                              ] else if (taqbinPayuniStatus.value ==
                                  payuniSubmitFailed) ...[
                                if (taqbinProviderMessage.value.isNotEmpty) ...[
                                  Text(
                                    '開通失敗：${taqbinProviderMessage.value}',
                                    style: TextStyle(
                                      fontSize: 14.sp,
                                      color: AppColors.error,
                                      height: 1.5,
                                    ),
                                  ),
                                  SizedBox(height: 8.h),
                                ],
                                SizedBox(
                                  width: double.infinity,
                                  height: 52,
                                  child: FilledButton(
                                    style: FilledButton.styleFrom(
                                      backgroundColor: AppColors.error,
                                    ),
                                    onPressed: (saving.value || !hasTaqbin)
                                        ? null
                                        : onActivateTaqbin,
                                    child: saving.value
                                        ? const SizedBox.square(
                                            dimension: 20,
                                            child: CircularProgressIndicator(
                                              strokeWidth: 2,
                                              color: AppColors.neutralWhite,
                                            ),
                                          )
                                        : const HkButtonLabel('重新嘗試'),
                                  ),
                                ),
                                SizedBox(height: 16.h),
                              ] else if (!taqbinLocked.value) ...[
                                SaveButton(
                                  canSave: hasTaqbin,
                                  saving: saving.value,
                                  onSave: onActivateTaqbin,
                                  buttonText: '開通黑貓物流',
                                ),
                                SizedBox(height: 16.h),
                              ],
                            ],

                            CvsShippingSection(
                              expanded: cvsExpanded,
                              onToggleExpanded: toggleCvsExpanded,
                              enabled: cvsEnabled.value,
                              senderLocked: cvsSenderLocked.value,
                              selected: cvsSelected,
                              nameCtrl: cvsNameCtrl,
                              phoneCtrl: cvsPhoneCtrl,
                              selectedStoreName: selectedStoreName,
                              selectedStoreCode: selectedStoreCode,
                              onPickStore: pickStore,
                            ),
                            const SizedBox(height: HkSpace.lg),

                            if (cvsExpanded.value)
                              if (cvsProcessing.value) ...[
                                const SaveButton(
                                  canSave: false,
                                  saving: true,
                                  buttonText: '處理中…',
                                ),
                                SizedBox(height: 16.h),
                              ] else if (!cvsEnabled.value || cvsDirty) ...[
                                SaveButton(
                                  canSave: cvsEnabled.value || hasCvs,
                                  saving: saving.value,
                                  onSave: onActivateCvs,
                                  buttonText: cvsEnabled.value
                                      ? '更新7-11物流'
                                      : '開通7-11物流',
                                ),
                                SizedBox(height: 16.h),
                              ],
                            const SizedBox(height: HkSpace.section),
                          ],
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
    );
  }
}
