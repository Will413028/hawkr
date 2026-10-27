"use client";

import { useEffect, useState } from "react";

/**
 * Track the user's reduced-motion preference so animated disclosures can
 * degrade to a plain expand for people who opt out.
 */
export default function useReducedMotion(): boolean {
	const [reduced, setReduced] = useState(false);
	useEffect(() => {
		const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
		const update = () => setReduced(mq.matches);
		update();
		mq.addEventListener("change", update);
		return () => mq.removeEventListener("change", update);
	}, []);
	return reduced;
}
