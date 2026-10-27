import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart'
    as common;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import 'address_model.dart';

part 'address_repository.g.dart';

@riverpod
AddressRepository addressRepository(Ref ref) => AddressRepository(
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
  defaults: ref.watch(defaultErrorTextProvider),
);

class AddressRepository {
  final BuyerAppServiceClient _buyer;
  final DefaultErrorText _defaults;
  AddressRepository(this._buyer, {required DefaultErrorText defaults})
    : _defaults = defaults;

  Address _model(buyer.Address address) => Address(
    id: int.parse(address.id),
    etag: address.etag,
    receiverName: address.receiverName,
    receiverPhone: address.receiverPhone,
    townCode: address.postalAddress.townCode,
    addressDetail: address.postalAddress.addressDetail,
    isDefault: address.defaultAddress,
  );

  Future<T> _call<T>(Future<T> Function() action) async {
    try {
      return await action();
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

  /// 每位買家的地址上限（伺服器在新增時驗證），平常一頁就是完整清單。
  static const addressLimit = 20;

  /// 取得全部地址：以 [addressLimit] 筆一頁請求；伺服器回了 `next_page_token`
  /// （例如上限調高）就接著讀，不無聲截斷。
  Future<List<Address>> getAddresses() => _call(() async {
    final addresses = <Address>[];
    var pageToken = '';
    do {
      final response = await _buyer.listAddresses(
        buyer.ListAddressesRequest(
          pageSize: addressLimit,
          pageToken: pageToken,
        ),
      );
      addresses.addAll(response.addresses.map(_model));
      pageToken = response.nextPageToken;
    } while (pageToken.isNotEmpty);
    return addresses;
  });

  Future<Address?> getDefaultAddress() async {
    final addresses = await getAddresses();
    if (addresses.isEmpty) return null;
    for (final address in addresses) {
      if (address.isDefault) return address;
    }
    return addresses.first;
  }

  Future<void> addAddress({
    required String receiverName,
    required String receiverPhone,
    required String townCode,
    required String addressDetail,
    bool isDefault = false,
  }) => _call(() async {
    await _buyer.createAddress(
      buyer.CreateAddressRequest(
        address: buyer.Address(
          receiverName: receiverName,
          receiverPhone: receiverPhone,
          postalAddress: common.PostalAddress(
            townCode: townCode,
            addressDetail: addressDetail,
          ),
          defaultAddress: isDefault,
        ),
      ),
    );
  });

  Future<void> updateAddress(
    int id, {
    required String etag,
    String? receiverName,
    String? receiverPhone,
    String? townCode,
    String? addressDetail,
    bool? isDefault,
  }) => _call(() async {
    final fields = <String>[];
    if (receiverName != null) fields.add('receiver_name');
    if (receiverPhone != null) fields.add('receiver_phone');
    // 鄉鎮市區代碼與明細一起修改：整個 postal_address 換掉，兩者都要有值。
    if ((townCode == null) != (addressDetail == null)) {
      throw const ServerException('地址的鄉鎮市區與明細必須一起修改');
    }
    if (townCode != null && townCode.isEmpty) {
      throw const ServerException('請選擇鄉鎮市區');
    }
    if (townCode != null) fields.add('postal_address');
    if (isDefault != null) fields.add('default_address');
    final address = buyer.Address(id: id.toString(), etag: etag);
    if (receiverName != null) address.receiverName = receiverName;
    if (receiverPhone != null) address.receiverPhone = receiverPhone;
    if (townCode != null && addressDetail != null) {
      address.postalAddress = common.PostalAddress(
        townCode: townCode,
        addressDetail: addressDetail,
      );
    }
    if (isDefault != null) address.defaultAddress = isDefault;
    final request = buyer.UpdateAddressRequest(
      address: address,
      updateMask: fieldMask(address, fields),
    );
    await _buyer.updateAddress(request);
  });

  Future<void> deleteAddress(int id) => _call(() async {
    await _buyer.deleteAddress(buyer.DeleteAddressRequest(id: id.toString()));
  });

  Future<void> setDefaultAddress(int id, {required String etag}) =>
      updateAddress(id, etag: etag, isDefault: true);
}
