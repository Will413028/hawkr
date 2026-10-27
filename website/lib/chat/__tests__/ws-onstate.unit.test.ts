import { describe, expect, it } from "vitest";
import { ChatSocket } from "../ws";

describe("ChatSocket.onState", () => {
	// Regression: NotificationToastBridge connects the singleton app-wide, so by
	// the time ThreadClient mounts the socket is already "connected" and
	// connect() emits nothing. A future-only onState left the thread's local
	// connectionState stuck at its initial "disconnected" → false
	// "網路連線異常" banner over a live socket. A new subscriber must get the
	// current state immediately.
	it("emits the current state to a new subscriber on subscribe", () => {
		const sock = new ChatSocket();
		const seen: string[] = [];
		sock.onState((s) => seen.push(s));
		expect(seen).toEqual(["disconnected"]);
	});
});
