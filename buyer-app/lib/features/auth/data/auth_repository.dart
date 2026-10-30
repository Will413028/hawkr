import 'package:hawkr_api/images.dart';
import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart'
    as identity;
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../core/exceptions/app_exception.dart';

part 'auth_repository.g.dart';

@riverpod
AuthRepository authRepository(Ref ref) => AuthRepository(
  ref.watch(identityClientProvider),
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
  defaults: ref.watch(defaultErrorTextProvider),
);

class AuthRepository {
  final IdentityAppServiceClient _identity;
  final BuyerAppServiceClient _buyer;
  final DefaultErrorText _defaults;

  AuthRepository(
    this._identity,
    this._buyer, {
    required DefaultErrorText defaults,
  }) : _defaults = defaults;

  // ─────────────────── 帳號（IdentityAppService） ───────────────────

  /// 目前登入的帳號 ID（GetSession）。Clerk 登入之後呼叫：第一次登入的使用者，伺服器在這時建立帳號。
  /// 錯誤轉成 [AppException]：文字取自伺服器的 LocalizedMessage（例如停權）。
  Future<String> currentAccountId() async {
    final res = await _call(
      'getSession',
      () => _identity.getSession(identity.GetSessionRequest()),
    );
    if (res.account.id.isEmpty) {
      throw const MalformedAuthResponse('GetSession without account id');
    }
    return res.account.id;
  }

  /// 刪除帳號（DeleteAccount）。失敗拋出原本的例外，由呼叫端依 [ErrorFacts] 分類：沒收到回應時帳號可能已刪除，
  /// 不能自動重送，要以 [accountExists] 確認。
  Future<identity.DeleteAccountResponse_DeletionState> deleteAccount() async {
    final response = await _identity.deleteAccount(
      identity.DeleteAccountRequest(),
    );
    return response.state;
  }

  /// 以 GetSession 確認帳號還在：有回應是 true，unauthenticated（token 已失效）是 false。其他失敗拋出原本的例外。
  Future<bool> accountExists() async {
    try {
      await _identity.getSession(identity.GetSessionRequest());
      return true;
    } catch (raw) {
      if (ConnectException.from(raw).code == Code.unauthenticated) return false;
      rethrow;
    }
  }

  /// 呼叫 identity 的方法，錯誤轉成 App 的 [AppException]：沒收到回應是 [NetworkException]，token 無效
  /// （unauthenticated）是 [UnauthorizedException]，其他是 [ServerException]。文字取自伺服器的
  /// LocalizedMessage，沒有時用該 code 的預設文字。取消當成 internal（見 `errorText`）。
  Future<T> _call<T>(String tag, Future<T> Function() call) async {
    try {
      return await call();
    } catch (raw) {
      if (raw is AppException) rethrow;
      final error = ConnectException.from(raw);
      HKLog.logE('[AuthRepository] $tag failed: ${error.code.name}', error);
      if (ErrorFacts.fromException(error).noResponse) {
        throw const NetworkException();
      }
      final message = errorText(error, defaults: _defaults);
      if (isSessionUnavailableCode(error.code)) {
        throw NetworkException(message);
      }
      if (error.code == Code.unauthenticated) {
        throw UnauthorizedException(message);
      }
      throw SuspendedException(message);
    }
  }

  // ─────────────────── Profile / media ───────────────────

  Future<T> _writeWithEtag<T>(Future<T> Function() write) async {
    try {
      return await write();
    } catch (raw) {
      final error = ConnectException.from(raw);
      if (error.code == Code.aborted) {
        throw ServerException(errorText(error, defaults: _defaults));
      }
      rethrow;
    }
  }

  /// 取得當前買家個人資料；讀不到時回 null（呼叫端把它當成沒有資料）。
  Future<BuyerProfile?> tryGetProfile() async {
    try {
      return await getProfile();
    } catch (_) {
      return null;
    }
  }

  Future<BuyerProfile> getProfile() async {
    try {
      final response = await _buyer.getProfile(buyer.GetProfileRequest());
      if (!response.hasProfile()) throw const ServerException('買家資料不存在');
      return _toBuyerProfile(response.profile);
    } catch (raw) {
      if (raw is AppException) rethrow;
      final error = ConnectException.from(raw);
      if (error.code == Code.unauthenticated) {
        throw const UnauthorizedException();
      }
      if (ErrorFacts.fromException(error).noResponse) {
        throw const NetworkException();
      }
      throw ServerException(errorText(error, defaults: _defaults));
    }
  }

  Future<void> updateProfile(ProfileEdit edit) async {
    final profile = buyer.Profile(
      id: edit.id,
      etag: edit.etag,
      displayName: edit.displayName,
      phone: edit.phone,
      city: edit.city,
      job: edit.job,
    );
    final gender = edit.gender;
    if (gender != null) profile.gender = gender;
    final birthDate = edit.birthDate;
    if (birthDate != null) profile.birthDate = birthDate;
    final request = buyer.UpdateProfileRequest(
      profile: profile,
      updateMask: fieldMask(profile, _profileEditPaths),
    );
    await _writeWithEtag(() => _buyer.updateProfile(request));
  }

  /// 換大頭貼：UpdateProfile 帶 `avatar_staged_token`（INPUT_ONLY）並列進 update mask，
  /// etag 用載入時的版本。後端把暫存圖片轉正並回傳新的頭像；務必用回傳值，
  /// 不要存上傳當下的 previewUrl（tmp/ 暫時簽章 URL 會過期失效）。
  /// 空的 token 不送出：進了 mask 的空 token 會被後端當成錯誤。
  Future<String> updateAvatar({
    required String id,
    required String stagedToken,
    required String etag,
  }) async {
    if (etag.isEmpty || id.isEmpty) {
      throw const ServerException('個人資料版本資訊不存在，請重新載入');
    }
    if (stagedToken.isEmpty) {
      throw const ServerException('頭像上傳尚未完成');
    }
    final profile = buyer.Profile(
      id: id,
      etag: etag,
      avatarStagedToken: stagedToken,
    );
    final request = buyer.UpdateProfileRequest(
      profile: profile,
      updateMask: fieldMask(profile, ['avatar_staged_token']),
    );
    final result = await _writeWithEtag(() => _buyer.updateProfile(request));
    return result.profile.hasAvatar()
        ? result.profile.avatar.uri(ImagePreset.avatar)
        : '';
  }

  BuyerProfile _toBuyerProfile(buyer.Profile profile) => BuyerProfile(
    id: profile.id,
    etag: profile.etag,
    displayName: profile.displayName,
    avatarUrl: profile.hasAvatar()
        ? profile.avatar.uri(ImagePreset.avatar)
        : '',
    phone: profile.phone,
    // optional enum：不認得的值與沒有設值無法分辨（API 規範），UNSPECIFIED 同樣當作沒有填。
    gender: profile.hasGender() && profile.gender != Gender.GENDER_UNSPECIFIED
        ? profile.gender
        : null,
    birthDate: profile.hasBirthDate() ? profile.birthDate : null,
    city: profile.city,
    job: profile.job,
  );
}

/// 買家的個人資料（讀取模型）。
final class BuyerProfile {
  const BuyerProfile({
    required this.id,
    required this.etag,
    required this.displayName,
    required this.avatarUrl,
    required this.phone,
    required this.gender,
    required this.birthDate,
    required this.city,
    required this.job,
  });

  final String id;
  final String etag;
  final String displayName;

  /// 沒有頭像時是空字串，呼叫端顯示預設頭像。
  final String avatarUrl;
  final String phone;

  /// null 是買家沒有填。
  final Gender? gender;
  final Date? birthDate;
  final String city;
  final String job;
}

/// 個人資料編輯表單的結果。每個欄位都是表單擁有的，送出時一律列進 update_mask；
/// null 是沒有值，送出時清除。
typedef ProfileEdit = ({
  String id,
  String etag,
  String displayName,
  String phone,
  String city,
  String job,
  Gender? gender,
  Date? birthDate,
});

const _profileEditPaths = [
  'display_name',
  'phone',
  'city',
  'job',
  'gender',
  'birth_date',
];

/// 伺服器的回應不符契約時拋出。
/// **不要在 UI 層 catch 後吞掉** — 這代表 backend spec drift，
/// 必須立即冒泡讓 QA / 後端看到。
class MalformedAuthResponse implements Exception {
  const MalformedAuthResponse(this.detail);
  final String detail;

  @override
  String toString() => 'MalformedAuthResponse: $detail';
}
