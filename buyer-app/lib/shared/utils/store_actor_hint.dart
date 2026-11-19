import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;

/// 以店家身分留言或回覆時的輸入框提示，帶店名（C2、C3）；[actor] 不是有名稱的店家時回 null，由呼叫端用一般提示。
String? storeActorHint(pb.Actor? actor, {required bool replying}) {
  if (actor == null ||
      actor.kind != pb.ActorKind.ACTOR_KIND_STORE ||
      !actor.hasName()) {
    return null;
  }
  final storeName = actor.name.trim();
  if (storeName.isEmpty) return null;
  return replying ? '以「$storeName」身分回覆…' : '以「$storeName」身分留言…';
}
