import 'dart:async';
import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/dio_provider.dart';
import 'shipping_sender.dart';

part 'tgos_address_service.g.dart';

/// TGOS 地址候選結果
class AddressSuggestion {
  final String fullAddress;
  final String county;
  final String district;
  final String detail;
  final double lat;
  final double lng;

  const AddressSuggestion({
    required this.fullAddress,
    required this.county,
    required this.district,
    required this.detail,
    required this.lat,
    required this.lng,
  });

  factory AddressSuggestion.fromJson(Map<String, dynamic> json) {
    return AddressSuggestion(
      fullAddress: json['fullAddress']?.toString() ?? '',
      county: json['county']?.toString() ?? '',
      district: json['district']?.toString() ?? '',
      detail: json['detail']?.toString() ?? '',
      lat: (json['lat'] as num?)?.toDouble() ?? 0,
      lng: (json['lng'] as num?)?.toDouble() ?? 0,
    );
  }
}

/// TGOS 地址驗證結果
class AddressValidation {
  final bool valid;
  final String? fullAddress;
  final String? county;
  final String? district;
  final String? detail;
  final String? lat;
  final String? lng;

  const AddressValidation({
    required this.valid,
    this.fullAddress,
    this.county,
    this.district,
    this.detail,
    this.lat,
    this.lng,
  });

  factory AddressValidation.fromJson(Map<String, dynamic> json) {
    return AddressValidation(
      valid: json['valid'] == true,
      fullAddress: json['fullAddress']?.toString(),
      county: json['county']?.toString(),
      district: json['district']?.toString(),
      detail: json['detail']?.toString(),
      lat: json['lat']?.toString(),
      lng: json['lng']?.toString(),
    );
  }
}

@riverpod
TgosAddressService tgosAddressService(Ref ref) =>
    TgosAddressService(ref.watch(dioProvider));

class TgosAddressService {
  final Dio _dio;

  TgosAddressService(this._dio);

  /// 模糊搜尋地址候選清單
  ///
  /// [county] 縣市（第一層下拉已選）
  /// [district] 鄉鎮市區（第一層下拉已選）
  /// [query] 路名+門牌（使用者輸入）
  /// [limit] 最多回傳幾筆，預設 8
  Future<List<AddressSuggestion>> getSuggestions({
    required String county,
    required String district,
    required String query,
    int limit = 8,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/api/v1/app/address/suggestions',
        queryParameters: {
          'county': county,
          'district': district,
          'query': query,
          'limit': limit,
        },
      );
      final body = resp.data;
      if (body == null || body['code'] != 0) return [];
      final list = body['data'] as List? ?? [];
      String norm(String s) => toShippingName(s).replaceAll('　', '');
      final normCounty = norm(county);
      final normDistrict = norm(district);
      return list
          .whereType<Map<String, dynamic>>()
          .map(AddressSuggestion.fromJson)
          .where(
            (s) =>
                norm(s.county) == normCounty &&
                norm(s.district) == normDistrict,
          )
          .toList();
    } catch (_) {
      return [];
    }
  }

  /// 嚴格驗證地址
  ///
  /// [address] 完整地址，例如「臺北市中正區中華路一段1號」
  Future<AddressValidation> validate(String address) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/api/v1/app/address/validate',
        queryParameters: {'address': address},
      );
      final body = resp.data;
      if (body == null || body['code'] != 0) {
        return const AddressValidation(valid: false);
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      return AddressValidation.fromJson(data);
    } catch (_) {
      return const AddressValidation(valid: false);
    }
  }
}
