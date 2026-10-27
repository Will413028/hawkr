interface DemoNoticeProps {
	/** 本頁示範的流程，例如「隱私政策」。 */
	topic: string;
}

/** 法律頁共用的作品集說明：Hawkr 是示範專案，不是實際營運的服務。 */
export default function DemoNotice({ topic }: DemoNoticeProps) {
	return (
		<>
			<p>Hawkr 是作品集的示範專案，不是實際營運的服務。</p>
			<ul>
				<li>不提供真實的交易、付款或配送。</li>
				<li>不收集、不處理真實的個人資料，請勿輸入真實資料。</li>
				<li>畫面中的商家、商品、訂單與評論都是虛構的示範資料。</li>
			</ul>
			<p>
				本頁只用來展示{topic}
				的流程與版面，不構成任何契約或法律義務。營運者：Hawkr 示範專案。
			</p>
		</>
	);
}
