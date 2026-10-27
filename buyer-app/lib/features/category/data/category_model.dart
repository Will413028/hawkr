/// Product category node from /public/categories/tree API.
class Category {
  final String id;
  final String name;
  final String? parentId;
  final String? parentName;
  final String? status;
  final String? icon;
  final List<Category> children;

  const Category({
    required this.id,
    required this.name,
    this.parentId,
    this.parentName,
    this.status,
    this.icon,
    this.children = const [],
  });

  factory Category.fromJson(Map<String, dynamic> json) => Category(
    id: (json['id'] ?? json['catId'] ?? '').toString(),
    name: (json['name'] ?? '').toString(),
    parentId: json['parentId']?.toString() ?? json['parentCid']?.toString(),
    parentName: json['parentName']?.toString(),
    status: json['status']?.toString(),
    icon: json['icon']?.toString(),
    children:
        (json['children'] as List?)
            ?.whereType<Map>()
            .map((e) => Category.fromJson(Map<String, dynamic>.from(e)))
            .toList() ??
        [],
  );
}
