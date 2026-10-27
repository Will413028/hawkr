import ContactPage from "@/features/content/contact/ContactPage";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata({
	title: "聯絡我們",
	description:
		"需要協助嗎？歡迎透過 Hawkr 的聯絡管道與我們聯繫。無論是訂單、付款與帳號相關的客服問題，賣家入駐與通路合作的洽談，或是商務、媒體與品牌合作等需求，你都能在這裡找到對應的服務窗口。我們會盡快回覆你的每一則問題、建議與合作邀請，陪你一起把產地到餐桌的事做得更好。",
	path: "/contact",
});

export default ContactPage;
