import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';

part 'contact_email.g.dart';

/// 帳號的聯絡信箱（identity 的 `GetContactEmail`／`SetContactEmail`）：賣家的收款帳戶聯絡人用它（D23 修訂）。
/// [verified] 為真是 provider 驗證過的信箱（唯讀，隨登入更新）；否則是賣家自己填的（未驗證，可以改）。
final class AccountContactEmail {
  const AccountContactEmail({required this.email, required this.verified});

  final String email;
  final bool verified;

  @override
  bool operator ==(Object other) =>
      other is AccountContactEmail &&
      other.email == email &&
      other.verified == verified;

  @override
  int get hashCode => Object.hash(email, verified);
}

/// 確認步驟的聯絡信箱欄位要不要給賣家輸入：只在帳號沒有 provider 驗證過的信箱時（沒有任何信箱，或只有自填的）。
bool contactEmailEditable(AccountContactEmail? contact) =>
    contact == null || !contact.verified;

/// 伺服器的格式規則：去前後空白、轉小寫後至多 254 字，`local@domain` 且網域要有點。
const contactEmailMaxLength = 254;
final _contactEmailPattern = RegExp(r'^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$');

String normalizeContactEmail(String value) => value.trim().toLowerCase();

/// 聯絡信箱輸入的錯誤文字；通過是 null。
String? validateContactEmail(String value) {
  final v = normalizeContactEmail(value);
  if (v.isEmpty) return '請輸入金流商聯絡信箱';
  if (!_contactEmailPattern.hasMatch(v)) return 'Email 格式不正確';
  if (v.runes.length > contactEmailMaxLength) {
    return '信箱最多 $contactEmailMaxLength 字';
  }
  return null;
}

final class ContactEmailRepository {
  const ContactEmailRepository(this._client);

  final IdentityAppServiceClient _client;

  AccountContactEmail? _of(ContactEmail? contact) {
    if (contact == null || contact.email.isEmpty) return null;
    return switch (contact.source) {
      ContactEmailSource.CONTACT_EMAIL_SOURCE_VERIFIED => AccountContactEmail(
        email: contact.email,
        verified: true,
      ),
      ContactEmailSource.CONTACT_EMAIL_SOURCE_SELF_ENTERED =>
        AccountContactEmail(email: contact.email, verified: false),
      _ => throw FormatException('不認得的聯絡信箱來源：${contact.source}'),
    };
  }

  /// 目前的聯絡信箱；帳號還沒有是 null。
  Future<AccountContactEmail?> fetch() async {
    final response = await _client.getContactEmail(GetContactEmailRequest());
    return _of(response.hasContactEmail() ? response.contactEmail : null);
  }

  /// 設定自填的聯絡信箱（帳號有 provider 驗證過的信箱時伺服器拒絕）。
  Future<AccountContactEmail> set(String email) async {
    final response = await _client.setContactEmail(
      SetContactEmailRequest(email: normalizeContactEmail(email)),
    );
    final contact = _of(
      response.hasContactEmail() ? response.contactEmail : null,
    );
    if (contact == null) throw const FormatException('聯絡信箱為空');
    return contact;
  }
}

@Riverpod(keepAlive: true)
ContactEmailRepository contactEmailRepository(Ref ref) =>
    ContactEmailRepository(ref.watch(identityClientProvider));

/// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。
@Riverpod(keepAlive: true, retry: noRetry)
Future<AccountContactEmail?> accountContactEmail(
  Ref ref,
  String accountId,
) async {
  try {
    return await ref.watch(contactEmailRepositoryProvider).fetch();
  } catch (error) {
    HKLog.logW('[ContactEmail] GetContactEmail failed: $error');
    rethrow;
  }
}

/// 目前登入帳號的聯絡信箱；沒有登入時是 null（外層）。
@Riverpod(keepAlive: true)
AsyncValue<AccountContactEmail?>? currentContactEmail(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(accountContactEmailProvider(accountId));
}

/// 重讀目前帳號的聯絡信箱。
@Riverpod(keepAlive: true)
class ContactEmailReader extends _$ContactEmailReader {
  @override
  void build() {}

  /// 重讀，完成後回傳有沒有成功。沒有登入時回 false。
  Future<bool> refresh() async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return false;
    ref.invalidate(accountContactEmailProvider(accountId));
    try {
      await ref.read(accountContactEmailProvider(accountId).future);
      return true;
    } catch (_) {
      return false;
    }
  }
}
