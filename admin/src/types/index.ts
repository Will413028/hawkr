export interface Merchant {
	id: number;
	shopName: string;
	status: number;
	isActive: boolean;
	certificationStatus?: number;
	logo?: string;
	contactName?: string;
	contactPhone?: string;
	region?: string;
	mainCategory?: string;
	createTime?: string;
}

export interface PaymentRequest {
	id: number;
	merchantId: number;
	merchantName: string;
	provider: "ecpay" | "newebpay";
	merchantIdOnProvider: string;
	status: number;
	applyTime: string;
}

export interface SysLog {
	id: number;
	username: string;
	operation: string;
	method: string;
	params: string;
	time: number;
	ip: string;
	createDate: string;
}

export interface CategoryNode {
	id: string;
	name: string;
	parentId: string;
	parentName: string;
	status: string;
	children?: CategoryNode[];
}
