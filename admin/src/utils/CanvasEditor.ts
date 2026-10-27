import {
	Canvas,
	Circle,
	FabricObject,
	Gradient,
	Image,
	IText,
	Line,
	Polygon,
	Rect,
	Triangle,
} from "fabric";

// fabric 7 把物件的 originX/originY 預設改成 center（升級指南的 breaking change）。
// 本編輯器所有定位（left/top、對齊、貼上位移）都以左上角為基準，沿用 fabric 6 的預設。
FabricObject.ownDefaults.originX = "left";
FabricObject.ownDefaults.originY = "top";

export interface LayerInfo {
	id: string;
	name: string;
	type: string;
	visible: boolean;
	locked: boolean;
}

export interface TextStyle {
	fontFamily?: string;
	fontSize?: number;
	fontWeight?: string;
	fontStyle?: string;
	fill?: string;
	textAlign?: string;
	underline?: boolean;
	linethrough?: boolean;
}

/** Fabric 物件上掛載的自訂中繼資料(id / name 非 fabric 內建型別) */
type FabricObjectWithMeta = FabricObject & { id?: string; name?: string };

export class CanvasEditor {
	private canvas: Canvas;
	private width: number;
	private height: number;
	private history: string[] = [];
	private historyIndex: number = -1;
	private maxHistory: number = 50;
	private clipboard: FabricObject | null = null;
	private onSelectionChange?: (obj: FabricObject | null) => void;
	private onLayersChange?: (layers: LayerInfo[]) => void;
	private boundKeyboardHandler = (e: KeyboardEvent) => this.handleKeyboard(e);

	constructor(canvasId: string, width: number = 800, height: number = 600) {
		this.width = width;
		this.height = height;

		this.canvas = new Canvas(canvasId, {
			width: width,
			height: height,
			backgroundColor: "#ffffff",
			preserveObjectStacking: true,
		});

		this.setupEventListeners();
		this.saveHistory();
	}

	private setupEventListeners() {
		this.canvas.on("selection:created", () => this.handleSelectionChange());
		this.canvas.on("selection:updated", () => this.handleSelectionChange());
		this.canvas.on("selection:cleared", () => this.handleSelectionChange());
		this.canvas.on("object:modified", () => {
			this.saveHistory();
			this.notifyLayersChange();
		});
		this.canvas.on("object:added", () => this.notifyLayersChange());
		this.canvas.on("object:removed", () => this.notifyLayersChange());

		// Keyboard shortcuts
		document.addEventListener("keydown", this.boundKeyboardHandler);
	}

	private handleKeyboard(e: KeyboardEvent) {
		if (
			e.target instanceof HTMLInputElement ||
			e.target instanceof HTMLTextAreaElement
		)
			return;

		const isMac = navigator.platform.toUpperCase().indexOf("MAC") >= 0;
		const ctrlKey = isMac ? e.metaKey : e.ctrlKey;

		if (ctrlKey && e.key === "z") {
			e.preventDefault();
			if (e.shiftKey) this.redo();
			else this.undo();
		} else if (ctrlKey && e.key === "c") {
			e.preventDefault();
			this.copy();
		} else if (ctrlKey && e.key === "v") {
			e.preventDefault();
			this.paste();
		} else if (ctrlKey && e.key === "d") {
			e.preventDefault();
			this.duplicate();
		} else if (e.key === "Delete" || e.key === "Backspace") {
			if (!(e.target instanceof HTMLInputElement)) {
				e.preventDefault();
				this.deleteActiveObject();
			}
		}
	}

	private handleSelectionChange() {
		const activeObj = this.canvas.getActiveObject();
		this.onSelectionChange?.(activeObj || null);
	}

	private notifyLayersChange() {
		this.onLayersChange?.(this.getLayers());
	}

	public setOnSelectionChange(callback: (obj: FabricObject | null) => void) {
		this.onSelectionChange = callback;
	}

	public setOnLayersChange(callback: (layers: LayerInfo[]) => void) {
		this.onLayersChange = callback;
	}

	// ===== History (Undo/Redo) =====
	private saveHistory() {
		const json = JSON.stringify(this.canvas.toJSON());
		if (this.historyIndex < this.history.length - 1) {
			this.history = this.history.slice(0, this.historyIndex + 1);
		}
		this.history.push(json);
		if (this.history.length > this.maxHistory) {
			this.history.shift();
		}
		this.historyIndex = this.history.length - 1;
	}

	public undo() {
		if (this.historyIndex > 0) {
			this.historyIndex--;
			this.loadFromHistory();
		}
	}

	public redo() {
		if (this.historyIndex < this.history.length - 1) {
			this.historyIndex++;
			this.loadFromHistory();
		}
	}

	private loadFromHistory() {
		const json = this.history[this.historyIndex];
		this.canvas.loadFromJSON(json).then(() => {
			this.canvas.requestRenderAll();
			this.notifyLayersChange();
		});
	}

	// ===== Clipboard =====
	public copy() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.clone().then((cloned: FabricObject) => {
				this.clipboard = cloned;
			});
		}
	}

	public paste() {
		if (!this.clipboard) return;
		this.clipboard.clone().then((cloned: FabricObject) => {
			cloned.set({
				left: (cloned.left || 0) + 20,
				top: (cloned.top || 0) + 20,
			});
			this.canvas.add(cloned);
			this.canvas.setActiveObject(cloned);
			this.saveHistory();
		});
	}

	public duplicate() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.clone().then((cloned: FabricObject) => {
				cloned.set({
					left: (cloned.left || 0) + 20,
					top: (cloned.top || 0) + 20,
				});
				this.canvas.add(cloned);
				this.canvas.setActiveObject(cloned);
				this.saveHistory();
			});
		}
	}

	// ===== Text =====
	public addText(text: string, style?: TextStyle) {
		const iText = new IText(text, {
			left: this.width / 2 - 100,
			top: this.height / 2 - 20,
			fill: style?.fill || "#333333",
			fontSize: style?.fontSize || 40,
			fontFamily: style?.fontFamily || "Arial",
			fontWeight: style?.fontWeight || "normal",
			fontStyle: (style?.fontStyle || "normal") as
				| "normal"
				| "italic"
				| "oblique",
			textAlign: (style?.textAlign || "left") as IText["textAlign"],
			underline: style?.underline || false,
			linethrough: style?.linethrough || false,
		});
		this.canvas.add(iText);
		this.canvas.setActiveObject(iText);
		this.saveHistory();
	}

	public addHeading() {
		this.addText("標題文字", { fontSize: 60, fontWeight: "bold" });
	}

	public addSubheading() {
		this.addText("副標題", { fontSize: 36 });
	}

	public addBodyText() {
		this.addText("內文文字", { fontSize: 24 });
	}

	public updateTextStyle(style: TextStyle) {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj && activeObj.type === "i-text") {
			activeObj.set(style as Partial<IText>);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	// ===== Shapes =====
	public addRect(options?: {
		fill?: string;
		stroke?: string;
		width?: number;
		height?: number;
	}) {
		const rect = new Rect({
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: options?.fill || "#4A90D9",
			stroke: options?.stroke,
			strokeWidth: options?.stroke ? 2 : 0,
			width: options?.width || 100,
			height: options?.height || 100,
			rx: 0,
			ry: 0,
		});
		this.canvas.add(rect);
		this.canvas.setActiveObject(rect);
		this.saveHistory();
	}

	public addRoundedRect() {
		const rect = new Rect({
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: "#4A90D9",
			width: 120,
			height: 80,
			rx: 15,
			ry: 15,
		});
		this.canvas.add(rect);
		this.canvas.setActiveObject(rect);
		this.saveHistory();
	}

	public addCircle(options?: { fill?: string; radius?: number }) {
		const circle = new Circle({
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: options?.fill || "#E74C3C",
			radius: options?.radius || 50,
		});
		this.canvas.add(circle);
		this.canvas.setActiveObject(circle);
		this.saveHistory();
	}

	public addTriangle(options?: { fill?: string }) {
		const triangle = new Triangle({
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: options?.fill || "#2ECC71",
			width: 100,
			height: 100,
		});
		this.canvas.add(triangle);
		this.canvas.setActiveObject(triangle);
		this.saveHistory();
	}

	public addLine() {
		const line = new Line([50, 50, 200, 50], {
			left: this.width / 2 - 75,
			top: this.height / 2,
			stroke: "#333333",
			strokeWidth: 3,
		});
		this.canvas.add(line);
		this.canvas.setActiveObject(line);
		this.saveHistory();
	}

	public addStar() {
		const points = this.createStarPoints(5, 50, 25);
		const star = new Polygon(points, {
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: "#F1C40F",
		});
		this.canvas.add(star);
		this.canvas.setActiveObject(star);
		this.saveHistory();
	}

	public addArrow() {
		const points = [
			{ x: 0, y: 20 },
			{ x: 60, y: 20 },
			{ x: 60, y: 0 },
			{ x: 100, y: 30 },
			{ x: 60, y: 60 },
			{ x: 60, y: 40 },
			{ x: 0, y: 40 },
		];
		const arrow = new Polygon(points, {
			left: this.width / 2 - 50,
			top: this.height / 2 - 30,
			fill: "#9B59B6",
		});
		this.canvas.add(arrow);
		this.canvas.setActiveObject(arrow);
		this.saveHistory();
	}

	public addHexagon() {
		const points = this.createPolygonPoints(6, 50);
		const hexagon = new Polygon(points, {
			left: this.width / 2 - 50,
			top: this.height / 2 - 50,
			fill: "#1ABC9C",
		});
		this.canvas.add(hexagon);
		this.canvas.setActiveObject(hexagon);
		this.saveHistory();
	}

	private createStarPoints(
		spikes: number,
		outerRadius: number,
		innerRadius: number,
	) {
		const points = [];
		const step = Math.PI / spikes;
		for (let i = 0; i < 2 * spikes; i++) {
			const radius = i % 2 === 0 ? outerRadius : innerRadius;
			const angle = i * step - Math.PI / 2;
			points.push({
				x: outerRadius + radius * Math.cos(angle),
				y: outerRadius + radius * Math.sin(angle),
			});
		}
		return points;
	}

	private createPolygonPoints(sides: number, radius: number) {
		const points = [];
		for (let i = 0; i < sides; i++) {
			const angle = (i * 2 * Math.PI) / sides - Math.PI / 2;
			points.push({
				x: radius + radius * Math.cos(angle),
				y: radius + radius * Math.sin(angle),
			});
		}
		return points;
	}

	// ===== Images =====
	public addImage(url: string) {
		Image.fromURL(url, { crossOrigin: "anonymous" })
			.then((img) => {
				const maxWidth = this.width * 0.6;
				const maxHeight = this.height * 0.6;
				const scale = Math.min(
					maxWidth / (img.width || 1),
					maxHeight / (img.height || 1),
					1,
				);

				img.set({
					left: this.width / 2 - ((img.width || 0) * scale) / 2,
					top: this.height / 2 - ((img.height || 0) * scale) / 2,
					scaleX: scale,
					scaleY: scale,
				});
				this.canvas.add(img);
				this.canvas.setActiveObject(img);
				this.saveHistory();
			})
			.catch((err) => {
				console.error("Failed to load image:", err);
			});
	}

	public addImageFromFile(file: File) {
		const reader = new FileReader();
		reader.onload = (e) => {
			const dataUrl = e.target?.result as string;
			this.addImage(dataUrl);
		};
		reader.readAsDataURL(file);
	}

	// ===== Object Properties =====
	public setObjectFill(color: string) {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set("fill", color);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public setObjectStroke(color: string, width: number = 2) {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set({ stroke: color, strokeWidth: width });
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public setObjectOpacity(opacity: number) {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set("opacity", opacity);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public setGradientFill(
		color1: string,
		color2: string,
		direction: "horizontal" | "vertical" = "horizontal",
	) {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			const gradient = new Gradient({
				type: "linear",
				coords:
					direction === "horizontal"
						? { x1: 0, y1: 0, x2: activeObj.width || 100, y2: 0 }
						: { x1: 0, y1: 0, x2: 0, y2: activeObj.height || 100 },
				colorStops: [
					{ offset: 0, color: color1 },
					{ offset: 1, color: color2 },
				],
			});
			activeObj.set("fill", gradient);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	// ===== Layers =====
	public getLayers(): LayerInfo[] {
		const objects = this.canvas.getObjects();
		return objects
			.map((obj, index) => ({
				id: (obj as FabricObjectWithMeta).id || `layer-${index}`,
				name: this.getObjectName(obj, index),
				type: obj.type || "object",
				visible: obj.visible !== false,
				locked: obj.selectable === false,
			}))
			.reverse();
	}

	private getObjectName(obj: FabricObject, index: number): string {
		const meta = obj as FabricObjectWithMeta;
		if (meta.name) return meta.name;
		switch (obj.type) {
			case "i-text":
				return `文字 ${index + 1}`;
			case "rect":
				return `矩形 ${index + 1}`;
			case "circle":
				return `圓形 ${index + 1}`;
			case "triangle":
				return `三角形 ${index + 1}`;
			case "polygon":
				return `多邊形 ${index + 1}`;
			case "line":
				return `線條 ${index + 1}`;
			case "image":
				return `圖片 ${index + 1}`;
			default:
				return `物件 ${index + 1}`;
		}
	}

	public selectLayerByIndex(index: number) {
		const objects = this.canvas.getObjects();
		const reversedIndex = objects.length - 1 - index;
		if (reversedIndex >= 0 && reversedIndex < objects.length) {
			this.canvas.setActiveObject(objects[reversedIndex]);
			this.canvas.requestRenderAll();
		}
	}

	public toggleLayerVisibility(index: number) {
		const objects = this.canvas.getObjects();
		const reversedIndex = objects.length - 1 - index;
		if (reversedIndex >= 0 && reversedIndex < objects.length) {
			const obj = objects[reversedIndex];
			obj.set("visible", !obj.visible);
			this.canvas.requestRenderAll();
			this.notifyLayersChange();
		}
	}

	public toggleLayerLock(index: number) {
		const objects = this.canvas.getObjects();
		const reversedIndex = objects.length - 1 - index;
		if (reversedIndex >= 0 && reversedIndex < objects.length) {
			const obj = objects[reversedIndex];
			const isLocked = obj.selectable === false;
			obj.set({
				selectable: isLocked,
				evented: isLocked,
			});
			this.canvas.requestRenderAll();
			this.notifyLayersChange();
		}
	}

	public moveLayerUp(index: number) {
		const objects = this.canvas.getObjects();
		const reversedIndex = objects.length - 1 - index;
		if (reversedIndex < objects.length - 1) {
			const obj = objects[reversedIndex];
			this.canvas.bringObjectForward(obj);
			this.saveHistory();
			this.notifyLayersChange();
		}
	}

	public moveLayerDown(index: number) {
		const objects = this.canvas.getObjects();
		const reversedIndex = objects.length - 1 - index;
		if (reversedIndex > 0) {
			const obj = objects[reversedIndex];
			this.canvas.sendObjectBackwards(obj);
			this.saveHistory();
			this.notifyLayersChange();
		}
	}

	public bringToFront() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			this.canvas.bringObjectToFront(activeObj);
			this.saveHistory();
			this.notifyLayersChange();
		}
	}

	public sendToBack() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			this.canvas.sendObjectToBack(activeObj);
			this.saveHistory();
			this.notifyLayersChange();
		}
	}

	// ===== Alignment =====
	public alignLeft() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set("left", 0);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public alignCenter() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set(
				"left",
				(this.width - (activeObj.width || 0) * (activeObj.scaleX || 1)) / 2,
			);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public alignRight() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set(
				"left",
				this.width - (activeObj.width || 0) * (activeObj.scaleX || 1),
			);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public alignTop() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set("top", 0);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public alignMiddle() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set(
				"top",
				(this.height - (activeObj.height || 0) * (activeObj.scaleY || 1)) / 2,
			);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public alignBottom() {
		const activeObj = this.canvas.getActiveObject();
		if (activeObj) {
			activeObj.set(
				"top",
				this.height - (activeObj.height || 0) * (activeObj.scaleY || 1),
			);
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	// ===== Background =====
	public setBackgroundColor(color: string) {
		this.canvas.backgroundColor = color;
		this.canvas.requestRenderAll();
		this.saveHistory();
	}

	public setBackgroundGradient(
		color1: string,
		color2: string,
		direction: "horizontal" | "vertical" = "vertical",
	) {
		const gradient = new Gradient({
			type: "linear",
			coords:
				direction === "horizontal"
					? { x1: 0, y1: 0, x2: this.width, y2: 0 }
					: { x1: 0, y1: 0, x2: 0, y2: this.height },
			colorStops: [
				{ offset: 0, color: color1 },
				{ offset: 1, color: color2 },
			],
		});
		this.canvas.backgroundColor = gradient as Canvas["backgroundColor"];
		this.canvas.requestRenderAll();
		this.saveHistory();
	}

	public setBackgroundImage(url: string) {
		Image.fromURL(url, { crossOrigin: "anonymous" }).then((img) => {
			const scaleX = this.width / (img.width || 1);
			const scaleY = this.height / (img.height || 1);
			img.set({ scaleX, scaleY });
			this.canvas.backgroundImage = img;
			this.canvas.requestRenderAll();
			this.saveHistory();
		});
	}

	// ===== Delete =====
	public deleteActiveObject() {
		const activeObject = this.canvas.getActiveObject();
		if (activeObject) {
			this.canvas.remove(activeObject);
			this.canvas.discardActiveObject();
			this.canvas.requestRenderAll();
			this.saveHistory();
		}
	}

	public clearAll() {
		this.canvas.clear();
		this.canvas.backgroundColor = "#ffffff";
		this.canvas.requestRenderAll();
		this.saveHistory();
	}

	// ===== Export =====
	public toDataURL(format: "png" | "jpeg" = "png", quality: number = 1) {
		return this.canvas.toDataURL({ format, quality, multiplier: 1 });
	}

	public toJSON() {
		return this.canvas.toJSON();
	}

	public loadFromJSON(json: string | object) {
		const data = typeof json === "string" ? JSON.parse(json) : json;
		return this.canvas.loadFromJSON(data).then(() => {
			this.canvas.requestRenderAll();
			this.saveHistory();
			this.notifyLayersChange();
		});
	}

	public downloadAsImage(
		filename: string = "banner",
		format: "png" | "jpeg" = "png",
	) {
		const dataUrl = this.toDataURL(format);
		const link = document.createElement("a");
		link.download = `${filename}.${format}`;
		link.href = dataUrl;
		link.click();
	}

	// ===== Canvas Properties =====
	public getActiveObject() {
		return this.canvas.getActiveObject();
	}

	public getCanvas() {
		return this.canvas;
	}

	public dispose() {
		document.removeEventListener("keydown", this.boundKeyboardHandler);
		this.canvas.dispose();
	}
}
