-- 2026-07-13：貼文互動類通知（按讚/留言/回覆）title 皆為死字，沒有動作者名字，
-- 只有 body 有 placeholder。比照 order.placed 的修法（20260713010000），title 一併
-- 帶入動作者暱稱。
--
-- 同時把 post.liked 的 liker_name（issue #1176 flip 成 snake_case 的殘留，唯一還在用
-- 舊 payload key 的貼文互動事件）統一改回 camelCase likerName，對齊其餘 comment.*
-- 事件（commenterName/replierName/likerName 皆為 camelCase，未被 #1176 動到）。
-- 對應 pkg/mq/events.go PostLikedEvent.LikerName json tag 同次改動，
-- tests/contract/golden/event_payload_keys.json 的 post.liked emitted_keys 同步更新。

UPDATE notification_templates
SET title_template = '#{likerName} 按讚了您的貼文',
    body_template = '#{likerName} 覺得您的貼文很讚！'
WHERE event_type = 'post.liked' AND target_user_type = 'seller';

UPDATE notification_templates
SET title_template = '#{commenterName} 在您的貼文留言了'
WHERE event_type = 'comment.created' AND target_user_type = 'seller';

UPDATE notification_templates
SET title_template = '#{commenterName} 在您的貼文留言了'
WHERE event_type = 'comment.created.buyer' AND target_user_type = 'buyer';

UPDATE notification_templates
SET title_template = '#{replierName} 回覆了您的留言'
WHERE event_type = 'comment.replied.seller' AND target_user_type = 'seller';

UPDATE notification_templates
SET title_template = '#{replierName} 回覆了您的留言'
WHERE event_type = 'comment.replied.buyer' AND target_user_type = 'buyer';

UPDATE notification_templates
SET title_template = '#{likerName} 讚了您的留言'
WHERE event_type = 'comment.like_changed' AND target_user_type = 'seller';
