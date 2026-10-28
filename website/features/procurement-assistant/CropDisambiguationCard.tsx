"use client";

import AppIcon from "@/components/shared/AppIcon";
import HawkrAgentMark from "./HawkrAgentMark";
import type { CropDisambiguation, CropDisambiguationOption } from "./types";

interface CropDisambiguationCardProps {
	disambiguation: CropDisambiguation;
	pending: boolean;
	onChoose: (option: CropDisambiguationOption) => void;
}

/**
 * 品項消歧問題卡（B-244 Ask 3.5）。跟 ClarifyQuestionCard 是同一組視覺
 * token——差別只在沒有跳過/自由輸入，callback 對象是 optionId
 * 而非 slotKey。故意不重新設計外觀，直接沿用 hawkr-agent-question-card* class。
 */
export default function CropDisambiguationCard({
	disambiguation,
	pending,
	onChoose,
}: CropDisambiguationCardProps) {
	return (
		<li className="hawkr-agent-question-row">
			<HawkrAgentMark
				label="Hawkr Agent"
				className="assistant-mark hawkr-agent-message-avatar hawkr-agent-message-avatar--agent"
			/>
			<section
				className="hawkr-agent-question-card"
				data-testid="crop-disambiguation-card"
				aria-labelledby="crop-disambiguation-prompt"
			>
				<header className="hawkr-agent-question-card__head">
					<h2 id="crop-disambiguation-prompt">{disambiguation.prompt}</h2>
					<p>選擇最接近的品項。</p>
				</header>
				<div className="hawkr-agent-question-card__controls">
					<fieldset
						className="hawkr-agent-question-card__options"
						aria-label="品項選項"
					>
						{disambiguation.options.map((option) => (
							<button
								key={option.optionId}
								type="button"
								disabled={pending}
								onClick={() => onChoose(option)}
								className="hawkr-agent-question-card__option"
							>
								<span>{option.label}</span>
								<span className="hawkr-agent-question-card__option-mark">
									<AppIcon name="check" size={16} aria-hidden="true" />
								</span>
							</button>
						))}
					</fieldset>
				</div>
			</section>
		</li>
	);
}
