import BrandLogo from "@/components/shared/BrandLogo";

export default function AboutHeroMascot() {
	return (
		<div className="about-hero-ip" data-about-hero-ip aria-hidden="true">
			<span className="about-hero-ip__glow" />
			<span className="about-hero-ip__image">
				<BrandLogo variant="mark" size="fill" />
			</span>
		</div>
	);
}
