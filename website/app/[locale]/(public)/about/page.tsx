import AboutPage from "@/features/about/AboutPage";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata({
	title: "關於我們",
	description:
		"Hawkr 相信食材不只該被上架，更該被理解。我們以產地直送、商家認證與交易保障，把產地到餐桌之間的信任重新接起來，讓每一位小農與職人的用心被看見，也讓你買得安心、吃得放心。從一籃當季蔬果到一條當日鮮魚，我們希望重新連結人與土地，讓你認識食物真正的價值與故事。",
	path: "/about",
});

export default AboutPage;
