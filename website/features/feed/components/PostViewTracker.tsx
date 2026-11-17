"use client";

import { useEffect } from "react";
import { recordPostView } from "@/lib/posts/repo";

/**
 * 單篇貼文檢視器開在某一篇時記一次瀏覽（`SocialPublicService.RecordPostView`）：匿名也能呼叫，
 * 登入者帶 session cookie 由伺服器以帳號去重；同一篇、同一個台北日、同一個觀看者只算一次，
 * 所以這裡不判斷登入狀態，也不自己去重。失敗不影響頁面。
 */
export default function PostViewTracker({ postId }: { postId: string }) {
	useEffect(() => {
		void recordPostView(postId);
	}, [postId]);
	return null;
}
