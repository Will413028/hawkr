import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/my_store_fixtures.dart';

const _name = SellerRequirement.SELLER_REQUIREMENT_STORE_NAME;
const _logo = SellerRequirement.SELLER_REQUIREMENT_LOGO;
const _town = SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE;
const _kyc = SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED;
const _active = SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE;
const _notSuspended = SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED;

OnboardingTask _task(List<OnboardingTask> tasks, OnboardingTaskId id) =>
    tasks.singleWhere((task) => task.id == id);

List<OnboardingTaskId> _ids(
  List<OnboardingTask> tasks,
  OnboardingTaskGroup group,
) => [
  for (final task in tasks)
    if (task.group == group) task.id,
];

void main() {
  group('公開前必須只由 exposure.missing_requirements 推導', () {
    test('缺店名、logo、所在地：商店資料列出缺少項目，認證通過', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_name, _logo, _town])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      final profile = _task(tasks, OnboardingTaskId.profile);
      expect(profile.completed, isFalse);
      expect(profile.statusLabel, '缺少：店名、logo、所在地');
      expect(profile.cta, '去設定');
      expect(_task(tasks, OnboardingTaskId.cert).completed, isTrue);
      expect(_task(tasks, OnboardingTaskId.cert).statusLabel, '已通過');
      expect(_task(tasks, OnboardingTaskId.cert).cta, isNull);
    });

    test('只缺 logo', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_logo, _kyc])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      );
      expect(_task(tasks, OnboardingTaskId.profile).statusLabel, '缺少：logo');
    });

    test('缺 KYC_APPROVED：狀態與動作來自流程狀態', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
      );
      final cert = _task(tasks, OnboardingTaskId.cert);
      expect(cert.completed, isFalse);
      expect(cert.statusLabel, '需補件');
      expect(cert.cta, '前往補件');
      expect(_task(tasks, OnboardingTaskId.profile).completed, isTrue);
      expect(_task(tasks, OnboardingTaskId.profile).statusLabel, '已完成');
    });

    test('讀不到流程狀態時和尚未開始一樣引導開始認證', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        flowStatus: null,
      );
      final cert = _task(tasks, OnboardingTaskId.cert);
      expect(cert.statusLabel, '未知');
      expect(cert.cta, '開始認證');
    });

    test('流程狀態是 UNSPECIFIED：狀態是目錄的「未知」，行動和尚未開始一樣', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
      );
      final cert = _task(tasks, OnboardingTaskId.cert);
      expect(cert.statusLabel, '未知');
      expect(cert.cta, '開始認證');
    });

    test('exposure 說已通過、流程狀態卻還沒讀到：以 exposure 為準', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: null,
      );
      expect(_task(tasks, OnboardingTaskId.cert).completed, isTrue);
    });

    test('停權與商店狀態列只在缺那個條件時出現', () {
      final none = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      );
      expect(_ids(none, OnboardingTaskGroup.beforePublish), [
        OnboardingTaskId.profile,
        OnboardingTaskId.cert,
      ]);

      final both = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing([_active, _notSuspended]),
        ),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      expect(_ids(both, OnboardingTaskGroup.beforePublish), [
        OnboardingTaskId.profile,
        OnboardingTaskId.cert,
        OnboardingTaskId.notSuspended,
        OnboardingTaskId.storeActive,
      ]);
      final suspended = _task(both, OnboardingTaskId.notSuspended);
      expect(suspended.title, '停權處分');
      expect(suspended.statusLabel, '停權中');
      expect(suspended.cta, '查看停權說明');
      final inactive = _task(both, OnboardingTaskId.storeActive);
      expect(inactive.title, '商店狀態');
      expect(inactive.statusLabel, '未營業');
      expect(inactive.cta, '查看說明');
      expect(suspended.completed, isFalse);
    });

    test('NOT_PENDING_DELETION 在顯示層併入「商店狀態／未營業」列，不出現其他公開條件', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing([
            SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
          ]),
        ),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      final ids = _ids(tasks, OnboardingTaskGroup.beforePublish);
      expect(ids, contains(OnboardingTaskId.storeActive));
      expect(ids, isNot(contains(OnboardingTaskId.otherRequirement)));
      expect(_task(tasks, OnboardingTaskId.storeActive).statusLabel, '未營業');
    });

    test('物流設定未回報：鎖定並顯示未營業', () {
      final shipping = _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(reportCapabilities: false),
          flowStatus: null,
        ),
        OnboardingTaskId.shipping,
      );
      expect(shipping.locked, isTrue);
      expect(shipping.statusLabel, '未營業');
    });

    test('物流設定缺 STORE_ACTIVE：鎖定並顯示未營業', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          shippingSetup: shippingSetupMissing([
            SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
          ]),
        ),
        flowStatus: null,
      );
      final shipping = _task(tasks, OnboardingTaskId.shipping);
      expect(shipping.locked, isTrue);
      expect(shipping.statusLabel, '未營業');
    });

    test('沒有 exposure（還沒讀到）：每個條件都當成還缺', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      expect(
        _task(tasks, OnboardingTaskId.profile).statusLabel,
        '缺少：店名、logo、所在地',
      );
      expect(_task(tasks, OnboardingTaskId.cert).completed, isFalse);
      expect(
        _ids(tasks, OnboardingTaskGroup.beforePublish),
        isNot(contains(OnboardingTaskId.otherRequirement)),
      );
    });

    test('完全沒有店家快照時也不當成已完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: null,
        flowStatus: null,
      );
      for (final task in tasks.where(
        (t) => t.group == OnboardingTaskGroup.beforePublish,
      )) {
        expect(task.completed, isFalse);
      }
    });

    test('全部滿足：公開前必須的列全部完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      final required = tasks.where(
        (t) => t.group == OnboardingTaskGroup.beforePublish,
      );
      expect(required.map((t) => t.id), [
        OnboardingTaskId.profile,
        OnboardingTaskId.cert,
      ]);
      expect(required.every((t) => t.completed), isTrue);
    });
  });

  group('建議完成', () {
    test('分組與順序', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      expect(_ids(tasks, OnboardingTaskGroup.recommended), [
        OnboardingTaskId.shipping,
        OnboardingTaskId.product,
        OnboardingTaskId.description,
      ]);
    });

    test('物流設定在 SHIPPING_SETUP 缺 KYC_APPROVED 時鎖定，與流程狀態無關', () {
      for (final status in [
        ...KYCFlowStatus.values.where(
          (s) => s != KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
        ),
        null,
      ]) {
        final tasks = buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(
            publicListing: publicListingMissing([_kyc]),
            shippingSetup: shippingSetupMissing([_kyc]),
          ),
          flowStatus: status,
        );
        final shipping = _task(tasks, OnboardingTaskId.shipping);
        expect(shipping.locked, isTrue, reason: '$status');
        expect(shipping.statusLabel, '需先通過認證');
      }
      expect(
        _task(
          buildOnboardingTasks(l: UiLocale.zhTW, my: null, flowStatus: null),
          OnboardingTaskId.shipping,
        ).locked,
        isTrue,
      );
    });

    test('物流設定只看 SHIPPING_SETUP：PUBLIC_LISTING 缺 KYC 也照 SHIPPING_SETUP 開放', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      );
      expect(_task(tasks, OnboardingTaskId.shipping).locked, isFalse);
    });

    test('物流設定缺停權或等待刪除：鎖定並顯示對應的既有文字', () {
      final suspended = _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(shippingSetup: shippingSetupMissing([_notSuspended])),
          flowStatus: null,
        ),
        OnboardingTaskId.shipping,
      );
      expect(suspended.locked, isTrue);
      expect(suspended.statusLabel, '未營業');

      final pending = _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(
            shippingSetup: shippingSetupMissing([
              SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
            ]),
          ),
          flowStatus: null,
        ),
        OnboardingTaskId.shipping,
      );
      expect(pending.locked, isTrue);
      expect(pending.statusLabel, '未營業');
    });

    test('exposure 已認證、GetMyKYC 讀不到：認證列已通過、物流解鎖，沒有矛盾', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: null,
      );
      expect(_task(tasks, OnboardingTaskId.cert).completed, isTrue);
      expect(_task(tasks, OnboardingTaskId.cert).statusLabel, '已通過');
      expect(_task(tasks, OnboardingTaskId.shipping).locked, isFalse);
    });

    test('列的語氣只分完成與未完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc, _notSuspended])),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
      );
      for (final task in tasks) {
        expect(
          task.tone,
          task.completed ? HkStatusTone.success : HkStatusTone.neutral,
          reason: '${task.id}',
        );
      }
    });

    test('APPROVED 之後依物流開通狀態顯示', () {
      OnboardingTask shippingOf(ShippingProgress progress) => _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(publicListing: publicListingMissing()),
          flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
          shipping: progress,
        ),
        OnboardingTaskId.shipping,
      );

      final todo = shippingOf(const ShippingProgress());
      expect(todo.locked, isFalse);
      expect(todo.statusLabel, '待設定');
      expect(todo.cta, '設定物流');

      final processing = shippingOf(const ShippingProgress(processing: true));
      expect(processing.statusLabel, '審核中');
      expect(processing.cta, '查看審核進度');

      final done = shippingOf(const ShippingProgress(done: true));
      expect(done.completed, isTrue);
      expect(done.statusLabel, '已開通');
      expect(done.cta, '查看物流設定');
    });

    test('物流列的 payuniSubmitStatus：4303 完成、4302 處理中、其餘都不算', () {
      expect(
        ShippingProgress.fromRows([
          {'payuniSubmitStatus': '4303'},
        ]).done,
        isTrue,
      );
      expect(
        ShippingProgress.fromRows([
          {'payuniSubmitStatus': '4302'},
        ]).processing,
        isTrue,
      );
      final other = ShippingProgress.fromRows([
        {'payuniSubmitStatus': '4301'},
        {'payuniSubmitStatus': '4304'},
        {'payuniSubmitStatus': ''},
        {},
      ]);
      expect(other.done, isFalse);
      expect(other.processing, isFalse);
    });

    test('商品與商店介紹', () {
      final empty = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      );
      expect(_task(empty, OnboardingTaskId.product).statusLabel, '待上架');
      expect(_task(empty, OnboardingTaskId.product).cta, '新增商品');
      expect(_task(empty, OnboardingTaskId.description).statusLabel, '待完成');

      final filled = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing(),
          description: '  產地直送  ',
        ),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
        productCount: 3,
      );
      expect(_task(filled, OnboardingTaskId.product).completed, isTrue);
      expect(_task(filled, OnboardingTaskId.product).statusLabel, '已上架');
      expect(_task(filled, OnboardingTaskId.description).completed, isTrue);
      expect(_task(filled, OnboardingTaskId.description).statusLabel, '已完成');
      expect(
        _task(
          buildOnboardingTasks(
            l: UiLocale.zhTW,
            my: myStore(
              publicListing: publicListingMissing(),
              description: '   ',
            ),
            flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
          ),
          OnboardingTaskId.description,
        ).completed,
        isFalse,
      );
    });

    test('商品數讀不到（null）當成還沒有商品', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
        productCount: null,
      );
      expect(_task(tasks, OnboardingTaskId.product).completed, isFalse);
    });
  });
}
