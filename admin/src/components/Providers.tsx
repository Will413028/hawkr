"use client";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { useState } from "react";
import { adminTransport } from "@/lib/connect/transport";
import { createAppQueryClient } from "@/lib/query/client";
import { AuthService } from "@/services/auth";

export default function Providers({ children }: { children: React.ReactNode }) {
	const [queryClient] = useState(createAppQueryClient);
	// connect-query 的讀寫（useQuery／useMutation 加上方法）走同源 BFF 的 transport；任何呼叫回
	// unauthenticated 都表示後端不再認得這位管理員，登出並回到 Access。
	const [transport] = useState(() =>
		adminTransport({ onUnauthenticated: AuthService.sessionExpired }),
	);
	return (
		<QueryClientProvider client={queryClient}>
			<TransportProvider transport={transport}>{children}</TransportProvider>
			{process.env.NODE_ENV === "development" && (
				<ReactQueryDevtools initialIsOpen={false} />
			)}
		</QueryClientProvider>
	);
}
