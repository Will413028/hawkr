import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/session/session_provider.dart';
import '../data/address_model.dart';
import '../data/address_repository.dart';

part 'address_controller.g.dart';

@Riverpod(keepAlive: true)
class AddressController extends _$AddressController {
  @override
  Future<List<Address>> build() async {
    ref.keepAlive();
    // Apple App Review 合規：訪客（無 token）打 address API 必 401，
    // 直接回空清單不打 API。登入後 session 變動會自動 rebuild 重新拉取。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const [];
    }
    return ref.watch(addressRepositoryProvider).getAddresses();
  }

  Future<void> add({
    required String receiverName,
    required String receiverPhone,
    required String townCode,
    required String addressDetail,
    bool isDefault = false,
  }) async {
    await ref
        .read(addressRepositoryProvider)
        .addAddress(
          receiverName: receiverName,
          receiverPhone: receiverPhone,
          townCode: townCode,
          addressDetail: addressDetail,
          isDefault: isDefault,
        );
    ref.invalidateSelf();
    await future;
  }

  Future<void> updateAddress(
    int id, {
    required String etag,
    String? receiverName,
    String? receiverPhone,
    String? townCode,
    String? addressDetail,
    bool? isDefault,
  }) async {
    await ref
        .read(addressRepositoryProvider)
        .updateAddress(
          id,
          etag: etag,
          receiverName: receiverName,
          receiverPhone: receiverPhone,
          townCode: townCode,
          addressDetail: addressDetail,
          isDefault: isDefault,
        );
    ref.invalidateSelf();
    await future;
  }

  Future<void> delete(int id) async {
    await ref.read(addressRepositoryProvider).deleteAddress(id);
    ref.invalidateSelf();
    await future;
  }

  Future<void> setDefault(int id) async {
    final addresses = await future;
    final address = addresses.where((item) => item.id == id).firstOrNull;
    if (address == null) throw StateError('地址不存在，請重新載入');
    await ref
        .read(addressRepositoryProvider)
        .setDefaultAddress(id, etag: address.etag);
    ref.invalidateSelf();
    await future;
  }
}
