import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    hide OnboardingTask;
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_seller/features/seller/data/payment_account.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/my_store_fixtures.dart';

const _name = SellerRequirement.SELLER_REQUIREMENT_STORE_NAME;
const _logo = SellerRequirement.SELLER_REQUIREMENT_LOGO;
const _town = SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE;
const _kyc = SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE;
const _active = SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE;
const _notSuspended = SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED;

OnboardingTask _task(List<OnboardingTask> tasks, SellerOnboardingTaskKey id) =>
    tasks.singleWhere((task) => task.id == id);

List<SellerOnboardingTaskKey> _ids(
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
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      final profile = _task(tasks, SellerOnboardingTaskKey.profile);
      expect(profile.completed, isFalse);
      expect(profile.statusLabel, '缺少：店名、logo、所在地');
      expect(profile.cta, '去設定');
      final review = _task(tasks, SellerOnboardingTaskKey.platformReview);
      expect(review.completed, isTrue);
      expect(review.statusLabel, '已通過');
      expect(review.cta, isNull);
    });

    test('只缺 logo', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_logo, _kyc])),
        reviewStatus: null,
        paymentAccount: null,
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.profile).statusLabel,
        '缺少：logo',
      );
    });

    test('缺 ONBOARDING_COMPLETE、平台審核還沒通過：平台審核列是申請狀態，收款帳戶列鎖定', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        paymentAccount: null,
      );
      final review = _task(tasks, SellerOnboardingTaskKey.platformReview);
      expect(review.completed, isFalse);
      expect(review.statusLabel, '審核中');
      expect(review.cta, '前往商家認證');
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.completed, isFalse);
      expect(payment.locked, isTrue);
      expect(payment.statusLabel, '平台審核通過後開放設定');
      expect(payment.cta, isNull);
      expect(_task(tasks, SellerOnboardingTaskKey.profile).completed, isTrue);
      expect(_task(tasks, SellerOnboardingTaskKey.profile).statusLabel, '已完成');
    });

    test('沒有申請或讀不到：平台審核列是「未送出」，引導去商家認證', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: null,
        paymentAccount: null,
      );
      final review = _task(tasks, SellerOnboardingTaskKey.platformReview);
      expect(review.statusLabel, '未送出');
      expect(review.cta, '前往商家認證');
    });

    test('GetMyKYC 還沒讀到或讀取失敗：平台審核列是目錄的「未知」，不顯示成「未送出」', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: null,
        reviewReadable: false,
        paymentAccount: null,
      );
      final review = _task(tasks, SellerOnboardingTaskKey.platformReview);
      expect(review.statusLabel, '未知');
      expect(review.cta, '前往商家認證');
    });

    test('收款帳戶已啟用：列是「已啟用」，完成而且沒有按鈕（不管店家的開通閘門讀到什麼）', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
        ),
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.statusLabel, '已啟用');
      expect(payment.completed, isTrue);
      expect(payment.cta, isNull);
    });

    test('平台審核還沒通過：就算讀到收款帳戶也不顯示它（effectivePaymentAccount 與文件一致，回 null）', () {
      final account = PaymentAccount(
        status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
      );
      expect(effectivePaymentAccount(account, approved: false), isNull);
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        paymentAccount: account,
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.completed, isFalse);
      expect(payment.locked, isTrue);
      expect(payment.statusLabel, '平台審核通過後開放設定');
    });

    test('收款帳戶讀取失敗（平台審核已通過）：不當成「準備中」，狀態是目錄的「未知」，按鈕去商家認證重讀', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
        paymentReadable: false,
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.statusLabel, '未知');
      expect(payment.completed, isFalse);
      expect(payment.cta, '前往商家認證');
    });

    test('收款帳戶讀取失敗但平台審核還沒通過：仍是「平台審核通過後開放設定」（讀取失敗只在核准之後才有意義）', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        paymentAccount: null,
        paymentReadable: false,
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.statusLabel, '平台審核通過後開放設定');
      expect(payment.locked, isTrue);
    });

    test('申請狀態是 UNSPECIFIED：狀態是目錄的「未知」', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_UNSPECIFIED,
        paymentAccount: null,
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.platformReview).statusLabel,
        '未知',
      );
    });

    test('平台審核通過、收款帳戶還沒有（外部帳戶在建立）：與「準備中」同義，動作是重新整理', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.locked, isFalse);
      expect(payment.completed, isFalse);
      expect(payment.statusLabel, '準備中');
      expect(payment.cta, '重新整理');
    });

    test('收款帳戶的狀態與動作都取自共用文案目錄：新帳戶是「待設定」，不說逾期或停用', () {
      PaymentAccount account(PaymentAccountStatus status) =>
          PaymentAccount(status: status);
      Map<PaymentAccountStatus, (String, String?)> expected = {
        PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED: (
          '準備中',
          '重新整理',
        ),
        PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW: (
          '審核中',
          '重新整理',
        ),
        PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED: (
          '待設定',
          '前往設定收款帳戶',
        ),
        PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED: ('無法啟用', '聯絡客服'),
      };
      for (final entry in expected.entries) {
        final tasks = buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(publicListing: publicListingMissing([_kyc])),
          reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
          paymentAccount: account(entry.key),
        );
        final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
        expect(payment.statusLabel, entry.value.$1, reason: entry.key.name);
        expect(payment.cta, entry.value.$2, reason: entry.key.name);
        expect(payment.completed, isFalse, reason: entry.key.name);
        expect(payment.statusLabel, isNot(contains('逾期')));
        expect(payment.statusLabel, isNot(contains('停用')));
      }
    });

    test('收款帳戶不認得的狀態（UNSPECIFIED）：狀態是目錄的「未知」，沒有動作', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: PaymentAccount(),
      );
      final payment = _task(tasks, SellerOnboardingTaskKey.paymentAccount);
      expect(payment.statusLabel, '未知');
      expect(payment.cta, isNull);
    });

    test('平台審核只讀申請狀態：流程狀態還沒讀到時，開通閘門已通過也不算平台審核完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        reviewStatus: null,
        reviewReadable: false,
        paymentAccount: null,
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.platformReview).completed,
        isFalse,
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.platformReview).statusLabel,
        '未知',
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.paymentAccount).completed,
        isFalse,
      );
    });

    test('停權與商店狀態列只在缺那個條件時出現', () {
      final none = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: null,
        paymentAccount: null,
      );
      expect(_ids(none, OnboardingTaskGroup.beforePublish), [
        SellerOnboardingTaskKey.profile,
        SellerOnboardingTaskKey.platformReview,
        SellerOnboardingTaskKey.paymentAccount,
      ]);

      final both = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing([_active, _notSuspended]),
        ),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      expect(_ids(both, OnboardingTaskGroup.beforePublish), [
        SellerOnboardingTaskKey.profile,
        SellerOnboardingTaskKey.platformReview,
        SellerOnboardingTaskKey.paymentAccount,
        SellerOnboardingTaskKey.notSuspended,
        SellerOnboardingTaskKey.storeActive,
      ]);
      final suspended = _task(both, SellerOnboardingTaskKey.notSuspended);
      expect(suspended.title, '停權處分');
      expect(suspended.statusLabel, '停權中');
      expect(suspended.cta, '查看停權說明');
      final inactive = _task(both, SellerOnboardingTaskKey.storeActive);
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
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      final ids = _ids(tasks, OnboardingTaskGroup.beforePublish);
      expect(ids, contains(SellerOnboardingTaskKey.storeActive));
      expect(ids, isNot(contains(SellerOnboardingTaskKey.otherRequirement)));
      expect(
        _task(tasks, SellerOnboardingTaskKey.storeActive).statusLabel,
        '未營業',
      );
    });

    test('物流設定未回報：鎖定並顯示未營業', () {
      final shipping = _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(reportCapabilities: false),
          reviewStatus: null,
          paymentAccount: null,
        ),
        SellerOnboardingTaskKey.shipping,
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
        reviewStatus: null,
        paymentAccount: null,
      );
      final shipping = _task(tasks, SellerOnboardingTaskKey.shipping);
      expect(shipping.locked, isTrue);
      expect(shipping.statusLabel, '未營業');
    });

    test('沒有 exposure（還沒讀到）：每個條件都當成還缺', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.profile).statusLabel,
        '缺少：店名、logo、所在地',
      );
      expect(
        _task(tasks, SellerOnboardingTaskKey.paymentAccount).completed,
        isFalse,
      );
      expect(
        _ids(tasks, OnboardingTaskGroup.beforePublish),
        isNot(contains(SellerOnboardingTaskKey.otherRequirement)),
      );
    });

    test('完全沒有店家快照時也不當成已完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: null,
        reviewStatus: null,
        paymentAccount: null,
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
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
        ),
      );
      final required = tasks.where(
        (t) => t.group == OnboardingTaskGroup.beforePublish,
      );
      expect(required.map((t) => t.id), [
        SellerOnboardingTaskKey.profile,
        SellerOnboardingTaskKey.platformReview,
        SellerOnboardingTaskKey.paymentAccount,
      ]);
      expect(required.every((t) => t.completed), isTrue);
    });
  });

  group('建議完成', () {
    test('分組與順序', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      expect(_ids(tasks, OnboardingTaskGroup.recommended), [
        SellerOnboardingTaskKey.shipping,
        SellerOnboardingTaskKey.product,
        SellerOnboardingTaskKey.description,
      ]);
    });

    test('物流設定在 SHIPPING_SETUP 缺 ONBOARDING_COMPLETE 時鎖定，與流程狀態無關', () {
      for (final status in [...KYCStatus.values, null]) {
        final tasks = buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(
            publicListing: publicListingMissing([_kyc]),
            shippingSetup: shippingSetupMissing([_kyc]),
          ),
          reviewStatus: status,
          paymentAccount: null,
        );
        final shipping = _task(tasks, SellerOnboardingTaskKey.shipping);
        expect(shipping.locked, isTrue, reason: '$status');
        expect(shipping.statusLabel, '需先通過認證');
      }
      expect(
        _task(
          buildOnboardingTasks(
            l: UiLocale.zhTW,
            my: null,
            reviewStatus: null,
            paymentAccount: null,
          ),
          SellerOnboardingTaskKey.shipping,
        ).locked,
        isTrue,
      );
    });

    test('物流設定只看 SHIPPING_SETUP：PUBLIC_LISTING 缺 KYC 也照 SHIPPING_SETUP 開放', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
        reviewStatus: null,
        paymentAccount: null,
      );
      expect(_task(tasks, SellerOnboardingTaskKey.shipping).locked, isFalse);
    });

    test('物流設定缺停權或等待刪除：鎖定並顯示對應的既有文字', () {
      final suspended = _task(
        buildOnboardingTasks(
          l: UiLocale.zhTW,
          my: myStore(shippingSetup: shippingSetupMissing([_notSuspended])),
          reviewStatus: null,
          paymentAccount: null,
        ),
        SellerOnboardingTaskKey.shipping,
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
          reviewStatus: null,
          paymentAccount: null,
        ),
        SellerOnboardingTaskKey.shipping,
      );
      expect(pending.locked, isTrue);
      expect(pending.statusLabel, '未營業');
    });

    test('物流解鎖只看伺服器的 SHIPPING_SETUP，與認證列各讀各的', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        reviewStatus: null,
        reviewReadable: false,
        paymentAccount: null,
      );
      expect(_task(tasks, SellerOnboardingTaskKey.shipping).locked, isFalse);
    });

    test('列的語氣只分完成與未完成', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc, _notSuspended])),
        reviewStatus: KYCStatus.KYC_STATUS_REJECTED,
        paymentAccount: null,
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
          reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
          paymentAccount: null,
          shipping: progress,
        ),
        SellerOnboardingTaskKey.shipping,
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
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
      );
      expect(_task(empty, SellerOnboardingTaskKey.product).statusLabel, '待上架');
      expect(_task(empty, SellerOnboardingTaskKey.product).cta, '新增商品');
      expect(
        _task(empty, SellerOnboardingTaskKey.description).statusLabel,
        '待完成',
      );

      final filled = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing(),
          description: '  產地直送  ',
        ),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
        productCount: 3,
      );
      expect(_task(filled, SellerOnboardingTaskKey.product).completed, isTrue);
      expect(_task(filled, SellerOnboardingTaskKey.product).statusLabel, '已上架');
      expect(
        _task(filled, SellerOnboardingTaskKey.description).completed,
        isTrue,
      );
      expect(
        _task(filled, SellerOnboardingTaskKey.description).statusLabel,
        '已完成',
      );
      expect(
        _task(
          buildOnboardingTasks(
            l: UiLocale.zhTW,
            my: myStore(
              publicListing: publicListingMissing(),
              description: '   ',
            ),
            reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
            paymentAccount: null,
          ),
          SellerOnboardingTaskKey.description,
        ).completed,
        isFalse,
      );
    });

    test('商品數讀不到（null）當成還沒有商品', () {
      final tasks = buildOnboardingTasks(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        reviewStatus: KYCStatus.KYC_STATUS_APPROVED,
        paymentAccount: null,
        productCount: null,
      );
      expect(_task(tasks, SellerOnboardingTaskKey.product).completed, isFalse);
    });
  });
}
