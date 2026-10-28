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
import '../../data/tgos_address_service.dart';
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

    // 黑貓宅急便狀態
    final taqbinExpanded = useState(true);
    final taqbinFrozen = useState(false);
    final taqbinChilled = useState(false);
    final taqbinAmbient = useState(false);
    final taqbinLocked = useState(false); // 寄件資料已交給 PayUni（4302、4303）
    // backend-payuni#666 四態：'' / '4301' NONE / '4302' REQUESTED / '4303' SUCCEEDED / '4304' FAILED
    final taqbinPayuniStatus = useState<String>('');
    final taqbinProviderMessage = useState<String>('');
    final homeNameCtrl = useTextEditingController();
    final homePhoneCtrl = useTextEditingController();
    final homeCity = useState<String?>(null);
    final homeDistrict = useState<String?>(null);
    final homeAddressCtrl = useTextEditingController();
    final homeAddressVerified = useState(false);
    final homeAddressVerifying = useState(false);

    // 7-11店到店狀態
    final cvsExpanded = useState(true);
    final cvsFrozen = useState(false);
    final cvsAmbient = useState(false);
    final cvsEnabled = useState(false); // 7-11是否已启用
    final cvsSenderLocked = useState(false); // 參考列已交給 PayUni（4302、4303）
    final cvsProcessing = useState(false); // 參考列 4302 處理中
    final cvsNameCtrl = useTextEditingController();
    final cvsPhoneCtrl = useTextEditingController();
    final selectedStoreName = useState<String?>(null);
    final selectedStoreCode = useState<String?>(null);
    final cvsInitialSnapshot = useState<String>('');
    // 已建店後增刪溫層改用 toggle endpoint，所以要記每個 goodsType 的 onboarding_id
    // 與初始啟用狀態，才能跟使用者目前勾選比對出 enable / disable。
    final cvsFrozenOnboardingId = useState<String?>(null);
    final cvsAmbientOnboardingId = useState<String?>(null);
    final cvsFrozenInitialEnabled = useState(false);
    final cvsAmbientInitialEnabled = useState(false);
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
        cvsFrozen.value ? '1' : '0',
        cvsAmbient.value ? '1' : '0',
        normalizeText(cvsNameCtrl.text),
        normalizeText(cvsPhoneCtrl.text),
        normalizeText(selectedStoreCode.value),
        normalizeText(selectedStoreName.value),
      ].join('|');
    }

    // 載入現有建店設定
    Future<void> loadData() async {
      loading.value = true;
      try {
        final bootstrap = await shippingController.loadBootstrap();
        HKLog.logD(
          '[ShippingSettings] onboardings loaded: '
          '${bootstrap.onboardings.length}',
        );
        // 同 website 的 findOnboarding：黑貓取第一筆，7-11 以常溫為準、沒有才用冷凍。
        Map<String, dynamic>? homeRow;
        Map<String, dynamic>? cvsAmbientRow;
        Map<String, dynamic>? cvsFrozenRow;
        var cvsToggleable = false;
        for (final o in bootstrap.onboardings) {
          final type = o['shippingType']?.toString() ?? '';
          final goods = o['goodsType']?.toString() ?? '';
          HKLog.logD(
            '[ShippingSettings] record: shippingType=$type goodsType=$goods '
            'payuniSubmitStatus=${o['payuniSubmitStatus']} '
            'onboardingStatus=${o['onboardingStatus']} senderName=${o['senderName']}',
          );
          // 後端回 codemeta value（3651=CVS_C2C, 3652=HOME_DELIVERY），
          // 也相容舊的字串 code 以防契約調整。
          final onboardingType =
              ShippingType.fromValue(type) ?? ShippingType.fromCode(type);
          if (onboardingType == ShippingType.homeDelivery) {
            homeRow ??= o;
          } else if (onboardingType == ShippingType.cvsC2c) {
            // 後端有兩個獨立維度：
            //  - payuniSubmitStatus 4303 且有 onboardingId = 已建店（一次性，跟 PayUni 對接結果）
            //  - enabled "true"/"false" = 各溫層 toggle 狀態（用 toggle endpoint 切換）
            // checkbox 應反映 enabled 字串，而非從 payuniSubmitStatus 推算。
            final actualEnabled = o['enabled']?.toString() == 'true';
            final onboardingId =
                o['onboardingId']?.toString() ??
                o['onboarding_id']?.toString() ??
                o['id']?.toString() ??
                '';
            if (goods == ShippingTemperature.frozen.value) {
              cvsFrozenRow ??= o;
              if (onboardingId.isNotEmpty) {
                cvsFrozenOnboardingId.value = onboardingId;
              }
              cvsFrozenInitialEnabled.value = actualEnabled;
              cvsFrozen.value = actualEnabled;
            }
            if (goods == ShippingTemperature.ambient.value) {
              cvsAmbientRow ??= o;
              if (onboardingId.isNotEmpty) {
                cvsAmbientOnboardingId.value = onboardingId;
              }
              cvsAmbientInitialEnabled.value = actualEnabled;
              cvsAmbient.value = actualEnabled;
            }
            // 已建店 → 後續 add/remove 走 toggle endpoint
            if (isShippingRowToggleable(o)) cvsToggleable = true;
          } else {
            // 🔴 本畫面只有「黑貓宅配」「7-11 店到店」兩張硬編卡片，
            // 其餘 ShippingType（3654 商家自送 / 後端新增值）
            // 沒有對應的區塊可以承載 —— 原本是**靜默忽略**（既不顯示也不報錯），
            // 這裡至少讓它在 log 留痕。
            // 真正的修法是 registry + slot，本波不做。
            HKLog.logW(
              '[ShippingSettings] 未支援的 onboarding shippingType=$type '
              '（本版僅支援 3651 CVS_C2C / 3652 HOME_DELIVERY），該筆設定不會顯示',
            );
          }
        }

        // 狀態、訊息、鎖定與寄件人都取自同一筆黑貓 onboarding。
        if (homeRow != null) {
          final payuniStatus = homeRow['payuniSubmitStatus']?.toString() ?? '';
          if (payuniStatus.isNotEmpty) taqbinPayuniStatus.value = payuniStatus;
          if (homeRow['providerMessage'] != null) {
            taqbinProviderMessage.value = homeRow['providerMessage'].toString();
          }
          // 只有 4302、4303 鎖定；失敗與其他狀態可以重新送出。
          if (isShippingRowLocked(homeRow)) {
            taqbinFrozen.value = true;
            taqbinChilled.value = true;
            taqbinAmbient.value = true;
            taqbinLocked.value = true;
          }
        }
        final cvsReference = cvsAmbientRow ?? cvsFrozenRow;
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

    // 寄件人：該物流商有紀錄就用紀錄，沒有才用店家。店家晚到也補上；
    // 每個物流商只套用一次，且只填使用者還沒輸入的欄位。
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

    // 門市選擇
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

    final hasTaqbin =
        taqbinFrozen.value || taqbinChilled.value || taqbinAmbient.value;
    final hasCvs = cvsFrozen.value || cvsAmbient.value;
    final cvsDirty = cvsInitialSnapshot.value != buildCvsSnapshot();
    void toggleTaqbinExpanded() {
      taqbinExpanded.value = !taqbinExpanded.value;
    }

    void toggleCvsExpanded() {
      cvsExpanded.value = !cvsExpanded.value;
    }

    // 開通黑貓物流
    Future<void> onActivateTaqbin() async {
      final confirmed = await showAppConfirmDialog(
        context,
        title: '確認開通黑貓物流？',
        message: '開通後，黑貓的溫層與收貨 / 退貨地址將不可在 App 內修改。',
        cancelText: '返回修改',
        confirmText: '確認開通',
      );
      if (!context.mounted) return;
      if (!confirmed) return;

      // 驗證黑貓必填（寄件人姓名顯示長度 ≤ 30，對齊 backend-payuni#1187 PayUni 約束）
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
      if (!homeAddressVerified.value) {
        FloatingToast.error(context, '請先驗證地址');
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
        // 黑貓建店（帶完整寄件人資訊）
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

    // 開通7-11物流
    //
    // 規則：
    // - 第一次啟用某個 goodsType（沒有 onboarding_id）→ POST /shipping_onboardings/cvs 建店
    // - 已建店後增刪溫層 → POST /shipping_onboardings/{onboarding_id}/toggle 切換啟用
    Future<void> onActivateCvs() async {
      if (saving.value || cvsMutationCoordinator.isRunning) return;

      // 驗證7-11必填
      if (selectedStoreCode.value == null || selectedStoreCode.value!.isEmpty) {
        FloatingToast.error(context, '請選擇退貨 7-11 門市');
        return;
      }

      // 驗證退貨收件人姓名顯示長度 ≤ 10（對齊 backend-payuni#1187 超商約束）。
      // 尚未開通（首次建店）時為必填；已開通的純 toggle 不送 senderName，
      // 故僅在有輸入時驗證長度上限。
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
            type: CvsGoodsType.frozen,
            goodsType: ShippingTemperature.frozen.value,
            initialEnabled: cvsFrozenInitialEnabled.value,
            targetEnabled: cvsFrozen.value,
            onboardingId: cvsFrozenOnboardingId.value,
            createBody: cvsBody,
          ),
          CvsGoodsTypeMutation(
            type: CvsGoodsType.ambient,
            goodsType: ShippingTemperature.ambient.value,
            initialEnabled: cvsAmbientInitialEnabled.value,
            targetEnabled: cvsAmbient.value,
            onboardingId: cvsAmbientOnboardingId.value,
            createBody: cvsBody,
          ),
        ]);

        if (!context.mounted) return;
        // Checkpoint each successful step immediately. If the next step fails,
        // rebuilding the plan from these values retries only the pending step.
        for (final applied in outcome.completed) {
          switch (applied.mutation.type) {
            case CvsGoodsType.frozen:
              cvsFrozenInitialEnabled.value = applied.mutation.targetEnabled;
              if (applied.onboardingId case final id?) {
                cvsFrozenOnboardingId.value = id;
              }
            case CvsGoodsType.ambient:
              cvsAmbientInitialEnabled.value = applied.mutation.targetEnabled;
              if (applied.onboardingId case final id?) {
                cvsAmbientOnboardingId.value = id;
              }
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

        final failedLabel = outcome.failedMutation?.label ?? '物流';
        final completedLabels = outcome.completed
            .map((item) => item.mutation.label)
            .join('、');
        final checkpointMessage = completedLabels.isEmpty
            ? ''
            : '$completedLabels已更新；';
        FloatingToast.error(
          context,
          '$checkpointMessage$failedLabel更新失敗：'
          '${outcome.error?.message ?? '請稍後再試'}。請重試未完成項目',
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
                              description: '選擇商品可使用的配送溫層；開通前請確認聯絡人與退貨資料。',
                            ),
                            const SizedBox(height: HkSpace.lg),

                            // 黑貓宅急便設定組件
                            TaqbinShippingSection(
                              expanded: taqbinExpanded,
                              onToggleExpanded: toggleTaqbinExpanded,
                              enabled: taqbinLocked.value,
                              frozen: taqbinFrozen,
                              chilled: taqbinChilled,
                              ambient: taqbinAmbient,
                              nameCtrl: homeNameCtrl,
                              phoneCtrl: homePhoneCtrl,
                              city: homeCity,
                              district: homeDistrict,
                              addressCtrl: homeAddressCtrl,
                              addressVerified: homeAddressVerified,
                              addressVerifying: homeAddressVerifying,
                              onCitySelected: (val) {
                                homeCity.value = val;
                                homeDistrict.value = null;
                                homeAddressVerified.value = false;
                              },
                              onDistrictSelected: (val) {
                                homeDistrict.value = val;
                                homeAddressVerified.value = false;
                              },
                              onAddressSelected: (suggestion) {
                                homeAddressVerified.value = false;
                              },
                              onAddressManualEdit: () {
                                homeAddressVerified.value = false;
                              },
                              onVerifyAddress: () async {
                                final addr = homeAddressCtrl.text.trim();
                                if (addr.isEmpty ||
                                    homeCity.value == null ||
                                    homeDistrict.value == null) {
                                  return;
                                }
                                homeAddressVerifying.value = true;
                                final tgos = ref.read(
                                  tgosAddressServiceProvider,
                                );
                                final fullAddr =
                                    '${homeCity.value}${homeDistrict.value}$addr';
                                final validation = await tgos.validate(
                                  fullAddr,
                                );
                                if (!context.mounted) return;
                                homeAddressVerifying.value = false;
                                String norm(String s) =>
                                    toShippingName(s).replaceAll('　', '');
                                final countyMatch =
                                    validation.county == null ||
                                    norm(validation.county!) ==
                                        norm(homeCity.value!);
                                final districtMatch =
                                    validation.district == null ||
                                    norm(validation.district!) ==
                                        norm(homeDistrict.value!);
                                if (validation.valid &&
                                    countyMatch &&
                                    districtMatch) {
                                  homeAddressVerified.value = true;
                                  FloatingToast.success(context, '地址驗證通過');
                                } else {
                                  homeAddressVerified.value = false;
                                  showDialog(
                                    context: context,
                                    builder: (dCtx) => Dialog(
                                      backgroundColor: Colors.transparent,
                                      insetPadding: const EdgeInsets.symmetric(
                                        horizontal: 48,
                                      ),
                                      child: Container(
                                        decoration: BoxDecoration(
                                          color: AppColors.bgCard,
                                          borderRadius: BorderRadius.circular(
                                            12,
                                          ),
                                        ),
                                        child: Column(
                                          mainAxisSize: MainAxisSize.min,
                                          children: [
                                            Padding(
                                              padding: EdgeInsets.fromLTRB(
                                                20.w,
                                                24.h,
                                                20.w,
                                                16.h,
                                              ),
                                              child: Column(
                                                children: [
                                                  Text(
                                                    '地址驗證失敗',
                                                    style: TextStyle(
                                                      fontSize: 16.sp,
                                                      fontWeight:
                                                          FontWeight.w700,
                                                      color:
                                                          AppColors.textPrimary,
                                                    ),
                                                  ),
                                                  SizedBox(height: 8.h),
                                                  Text(
                                                    '找不到此地址，請確認路名及門牌號碼是否正確後重新填寫。',
                                                    textAlign: TextAlign.center,
                                                    style: TextStyle(
                                                      fontSize: 14.sp,
                                                      color: AppColors
                                                          .textPrimary
                                                          .withValues(
                                                            alpha: 0.8,
                                                          ),
                                                      height: 1.5,
                                                    ),
                                                  ),
                                                ],
                                              ),
                                            ),
                                            Container(
                                              height: 1,
                                              color: AppColors.textSecondary,
                                            ),
                                            InkWell(
                                              onTap: () =>
                                                  Navigator.of(dCtx).pop(),
                                              child: SizedBox(
                                                height: 44.h,
                                                child: Center(
                                                  child: Text(
                                                    '重新填寫',
                                                    style: TextStyle(
                                                      fontSize: 16.sp,
                                                      fontWeight:
                                                          FontWeight.w700,
                                                      color:
                                                          AppColors.textPrimary,
                                                    ),
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

                              // backend-payuni#666 四態 button：
                              // 4303 SUCCEEDED → 不顯示（已開通）
                              // 4302 REQUESTED → 「處理中…」disabled + spinner
                              // 4304 FAILED → 顯示 PayUni 錯誤訊息 + 「重新嘗試」紅字按鈕
                              // 其他（4301 NONE / 舊資料無狀態）→ 「開通黑貓物流」
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
                                    onPressed:
                                        (saving.value ||
                                            !hasTaqbin ||
                                            !homeAddressVerified.value)
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
                                  canSave:
                                      hasTaqbin && homeAddressVerified.value,
                                  saving: saving.value,
                                  onSave: onActivateTaqbin,
                                  buttonText: '開通黑貓物流',
                                ),
                                SizedBox(height: 16.h),
                              ],
                            ],

                            // 7-11店到店設定組件
                            CvsShippingSection(
                              expanded: cvsExpanded,
                              onToggleExpanded: toggleCvsExpanded,
                              enabled: cvsEnabled.value,
                              senderLocked: cvsSenderLocked.value,
                              frozen: cvsFrozen,
                              ambient: cvsAmbient,
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
                                  // 已開通後，即使全部取消（disable 兩個溫層）也要允許 toggle save
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
