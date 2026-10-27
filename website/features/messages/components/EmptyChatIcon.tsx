import AppIcon from "@/components/shared/AppIcon";

/**
 * Icon used inside `.chat-empty-panel` when no conversation is selected.
 * 168×168 to match the previous 112px CSS gradient circle plus breathing room.
 */
export default function EmptyChatIcon({ size = 168 }: { size?: number }) {
	return <AppIcon name="chat" size={size} />;
}
