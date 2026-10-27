/**
 * Grainient shader adapted from React Bits at revision
 * 0e69e737242df1d257b4e5e399b01ae1d7901375. See
 * third_party/react-bits/LICENSE.md and third_party/react-bits/NOTICE.md.
 */
export type GrainientRenderer = {
	pause(): void;
	resume(): void;
	setDarkMode(dark: boolean): void;
	destroy(): void;
};

const MAX_RENDER_PIXELS = 320_000;
const MAX_DEVICE_PIXEL_RATIO = 0.75;
const FRAME_INTERVAL_MS = 1000 / 24;
const DARK_BRIGHTNESS = 0.12;

const VERTEX_SHADER_SOURCE = `#version 300 es
in vec2 position;
void main() {
  gl_Position = vec4(position, 0.0, 1.0);
}
`;

const FRAGMENT_SHADER_SOURCE = `#version 300 es
precision highp float;
uniform vec2 iResolution;
uniform float iTime;
uniform float uTimeSpeed;
uniform float uColorBalance;
uniform float uWarpStrength;
uniform float uWarpFrequency;
uniform float uWarpSpeed;
uniform float uWarpAmplitude;
uniform float uBlendAngle;
uniform float uBlendSoftness;
uniform float uRotationAmount;
uniform float uNoiseScale;
uniform float uGrainAmount;
uniform float uGrainScale;
uniform float uGrainAnimated;
uniform float uContrast;
uniform float uGamma;
uniform float uSaturation;
uniform vec2 uCenterOffset;
uniform float uZoom;
uniform vec3 uColor1;
uniform vec3 uColor2;
uniform vec3 uColor3;
uniform vec3 uColor4;
uniform float uLightMode;
uniform float uBrightness;
out vec4 fragColor;
#define S(a,b,t) smoothstep(a,b,t)
mat2 Rot(float a){float s=sin(a),c=cos(a);return mat2(c,-s,s,c);}
vec2 hash(vec2 p){p=vec2(dot(p,vec2(2127.1,81.17)),dot(p,vec2(1269.5,283.37)));return fract(sin(p)*43758.5453);}
float noise(vec2 p){vec2 i=floor(p),f=fract(p),u=f*f*(3.0-2.0*f);float n=mix(mix(dot(-1.0+2.0*hash(i+vec2(0.0,0.0)),f-vec2(0.0,0.0)),dot(-1.0+2.0*hash(i+vec2(1.0,0.0)),f-vec2(1.0,0.0)),u.x),mix(dot(-1.0+2.0*hash(i+vec2(0.0,1.0)),f-vec2(0.0,1.0)),dot(-1.0+2.0*hash(i+vec2(1.0,1.0)),f-vec2(1.0,1.0)),u.x),u.y);return 0.5+0.5*n;}
void mainImage(out vec4 o, vec2 C){
  float t=iTime*uTimeSpeed;
  vec2 uv=C/iResolution.xy;
  float ratio=iResolution.x/iResolution.y;
  vec2 tuv=uv-0.5+uCenterOffset;
  tuv/=max(uZoom,0.001);

  float degree=noise(vec2(t*0.1,tuv.x*tuv.y)*uNoiseScale);
  tuv.y*=1.0/ratio;
  tuv*=Rot(radians((degree-0.5)*uRotationAmount+180.0));
  tuv.y*=ratio;

  float frequency=uWarpFrequency;
  float ws=max(uWarpStrength,0.001);
  float amplitude=uWarpAmplitude/ws;
  float warpTime=t*uWarpSpeed;
  tuv.x+=sin(tuv.y*frequency+warpTime)/amplitude;
  tuv.y+=sin(tuv.x*(frequency*1.5)+warpTime)/(amplitude*0.5);

  float b=uColorBalance;
  float s=max(uBlendSoftness,0.0);
  mat2 blendRot=Rot(radians(uBlendAngle));
  float blendX=(tuv*blendRot).x;
  float edge0=-0.3-b-s;
  float edge1=0.2-b+s;
  float v0=0.5-b+s;
  float v1=-0.3-b-s;
  float horizontalBlend=S(edge0,edge1,blendX);
  // Reverse smoothstep edges are undefined in GLSL ES 3.00; invert ascending edges instead.
  float verticalBlend=1.0-S(v1,v0,tuv.y);
  // Bilinear corner interpolation keeps every transition between adjacent brand
  // colours; unlike layered gradients, no colour is additively stacked.
  vec3 top=mix(uColor1,uColor2,horizontalBlend);
  vec3 bottom=mix(uColor4,uColor3,horizontalBlend);
  vec3 col=mix(bottom,top,verticalBlend);

  vec2 grainUv=uv*max(uGrainScale,0.001);
  if(uGrainAnimated>0.5){grainUv+=vec2(iTime*0.05);}
  float grain=fract(sin(dot(grainUv,vec2(12.9898,78.233)))*43758.5453);
  col+=(grain-0.5)*uGrainAmount;

  col=(col-0.5)*uContrast+0.5;
  float luma=dot(col,vec3(0.2126,0.7152,0.0722));
  col=mix(vec3(luma),col,uSaturation);
  col=pow(max(col,0.0),vec3(1.0/max(uGamma,0.001)));
  col=clamp(col,0.0,1.0);
  if(uLightMode>0.5){
    float energy=max(max(col.r,col.g),col.b);
    vec3 hue=col/max(energy,0.001);
    float chroma=length(col-vec3(dot(col,vec3(0.333333))));
    float coverage=clamp(0.12+chroma*1.15+energy*0.18,0.0,0.88);
    col=mix(vec3(1.0),clamp(hue*0.58+col*0.18,0.0,1.0),coverage);
  }
  col=clamp(col*uBrightness,0.0,uBrightness);

  o=vec4(col,1.0);
}
void main(){
  vec4 o=vec4(0.0);
  mainImage(o,gl_FragCoord.xy);
  fragColor=o;
}
`;

type Rgb = readonly [number, number, number];

function parseHexColor(value: string): Rgb | null {
	// CSS optimizers legitimately shorten #ffffff to #fff, including tokens
	// read via getComputedStyle. Accept both forms rather than dropping to CSS.
	const expanded = value.replace(
		/^#?([\da-f])([\da-f])([\da-f])$/i,
		"#$1$1$2$2$3$3",
	);
	const match = /^#?([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i.exec(expanded);
	if (!match) return null;

	return [
		Number.parseInt(match[1], 16) / 255,
		Number.parseInt(match[2], 16) / 255,
		Number.parseInt(match[3], 16) / 255,
	];
}

function renderSize(
	cssWidth: number,
	cssHeight: number,
	devicePixelRatio: number,
): readonly [number, number] {
	const safeWidth = Number.isFinite(cssWidth) ? Math.max(cssWidth, 0) : 0;
	const safeHeight = Number.isFinite(cssHeight) ? Math.max(cssHeight, 0) : 0;
	const safeDpr = Number.isFinite(devicePixelRatio)
		? Math.min(Math.max(devicePixelRatio, 0.1), MAX_DEVICE_PIXEL_RATIO)
		: MAX_DEVICE_PIXEL_RATIO;
	let width = Math.max(1, Math.round(safeWidth * safeDpr));
	let height = Math.max(1, Math.round(safeHeight * safeDpr));

	if (width * height > MAX_RENDER_PIXELS) {
		const scale = Math.sqrt(MAX_RENDER_PIXELS / (width * height));
		width = Math.max(1, Math.floor(width * scale));
		height = Math.max(1, Math.floor(height * scale));
	}

	return [width, height];
}

export function createGrainientRenderer(
	canvas: HTMLCanvasElement,
	options: {
		colors: readonly [string, string, string, string];
		dark: boolean;
		onError: () => void;
	},
): GrainientRenderer | null {
	const parent = canvas.parentElement;
	const ownerWindow = canvas.ownerDocument.defaultView;
	const ResizeObserverConstructor = ownerWindow?.ResizeObserver;
	if (
		!parent ||
		!ownerWindow ||
		!ResizeObserverConstructor ||
		typeof ownerWindow.requestAnimationFrame !== "function" ||
		typeof ownerWindow.cancelAnimationFrame !== "function"
	) {
		return null;
	}

	const colors = options.colors.map(parseHexColor) as readonly (Rgb | null)[];
	if (colors.some((color) => color === null)) return null;

	let gl: WebGL2RenderingContext | null = null;
	try {
		gl = canvas.getContext("webgl2", {
			alpha: true,
			antialias: false,
			depth: false,
			stencil: false,
			premultipliedAlpha: true,
			preserveDrawingBuffer: false,
			powerPreference: "low-power",
			failIfMajorPerformanceCaveat: true,
		});
	} catch {
		return null;
	}
	if (!gl) return null;

	let vertexShader: WebGLShader | null = null;
	let fragmentShader: WebGLShader | null = null;
	let program: WebGLProgram | null = null;
	let vertexArray: WebGLVertexArrayObject | null = null;
	let vertexBuffer: WebGLBuffer | null = null;
	let resizeObserver: ResizeObserver | null = null;
	let animationFrame: number | null = null;
	let running = false;
	let destroyed = false;
	let errorNotified = false;
	let elapsedSeconds = 0;
	let lastTickMs: number | null = null;
	let lastDrawMs: number | null = null;

	const requestFrame = ownerWindow.requestAnimationFrame.bind(ownerWindow);
	const cancelFrame = ownerWindow.cancelAnimationFrame.bind(ownerWindow);
	const bindProgram = gl.useProgram.bind(gl);

	const safely = (operation: () => void) => {
		try {
			operation();
		} catch {
			// Cleanup remains best-effort if the WebGL context is already lost.
		}
	};

	const releaseResources = () => {
		if (animationFrame !== null) {
			const frameToCancel = animationFrame;
			safely(() => cancelFrame(frameToCancel));
			animationFrame = null;
		}
		const observerToDisconnect = resizeObserver;
		if (observerToDisconnect) {
			safely(() => observerToDisconnect.disconnect());
		}
		resizeObserver = null;
		// Deleting a current program defers its release. Unbind first, including
		// a VAO left current by a failed draw, before deleting GPU handles.
		safely(() => gl.bindVertexArray(null));
		safely(() => gl.bindBuffer(gl.ARRAY_BUFFER, null));
		safely(() => gl.useProgram(null));

		if (vertexArray) {
			safely(() => gl.deleteVertexArray(vertexArray));
			vertexArray = null;
		}
		if (vertexBuffer) {
			safely(() => gl.deleteBuffer(vertexBuffer));
			vertexBuffer = null;
		}
		const targetProgram = program;
		const targetVertexShader = vertexShader;
		const targetFragmentShader = fragmentShader;
		if (targetProgram && targetVertexShader) {
			safely(() => gl.detachShader(targetProgram, targetVertexShader));
		}
		if (targetProgram && targetFragmentShader) {
			safely(() => gl.detachShader(targetProgram, targetFragmentShader));
		}
		if (targetVertexShader) {
			safely(() => gl.deleteShader(targetVertexShader));
			vertexShader = null;
		}
		if (targetFragmentShader) {
			safely(() => gl.deleteShader(targetFragmentShader));
			fragmentShader = null;
		}
		if (targetProgram) {
			safely(() => gl.deleteProgram(targetProgram));
			program = null;
		}
	};

	const destroyInternal = () => {
		if (destroyed) return;
		destroyed = true;
		running = false;
		lastTickMs = null;
		lastDrawMs = null;
		canvas.removeEventListener("webglcontextlost", handleContextLost);
		releaseResources();
	};

	const failAfterReady = () => {
		if (destroyed || errorNotified) return;
		errorNotified = true;
		destroyInternal();
		try {
			options.onError();
		} catch {
			// The renderer must not leak an exception from an asynchronous failure.
		}
	};

	function handleContextLost() {
		failAfterReady();
	}

	const compileShader = (type: number, source: string): WebGLShader => {
		const shader = gl.createShader(type);
		if (!shader) throw new Error("Unable to create Grainient shader");
		try {
			gl.shaderSource(shader, source);
			gl.compileShader(shader);
			if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
				throw new Error("Unable to compile Grainient shader");
			}
			return shader;
		} catch (error) {
			safely(() => gl.deleteShader(shader));
			throw error;
		}
	};

	try {
		vertexShader = compileShader(gl.VERTEX_SHADER, VERTEX_SHADER_SOURCE);
		fragmentShader = compileShader(gl.FRAGMENT_SHADER, FRAGMENT_SHADER_SOURCE);

		program = gl.createProgram();
		if (!program) throw new Error("Unable to create Grainient program");
		gl.attachShader(program, vertexShader);
		gl.attachShader(program, fragmentShader);
		gl.bindAttribLocation(program, 0, "position");
		gl.linkProgram(program);
		if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
			throw new Error("Unable to link Grainient program");
		}

		vertexArray = gl.createVertexArray();
		vertexBuffer = gl.createBuffer();
		if (!vertexArray || !vertexBuffer) {
			throw new Error("Unable to create Grainient geometry");
		}

		gl.bindVertexArray(vertexArray);
		gl.bindBuffer(gl.ARRAY_BUFFER, vertexBuffer);
		gl.bufferData(
			gl.ARRAY_BUFFER,
			new Float32Array([-1, -1, 3, -1, -1, 3]),
			gl.STATIC_DRAW,
		);
		gl.enableVertexAttribArray(0);
		gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
		gl.bindVertexArray(null);
		gl.bindBuffer(gl.ARRAY_BUFFER, null);

		const uniform = (name: string): WebGLUniformLocation => {
			const location = gl.getUniformLocation(program as WebGLProgram, name);
			if (location === null) {
				throw new Error(`Unable to find Grainient uniform: ${name}`);
			}
			return location;
		};

		const uniforms = {
			resolution: uniform("iResolution"),
			time: uniform("iTime"),
			brightness: uniform("uBrightness"),
		};

		bindProgram(program);
		gl.uniform1f(uniform("uTimeSpeed"), 0.25);
		gl.uniform1f(uniform("uColorBalance"), 0);
		gl.uniform1f(uniform("uWarpStrength"), 1);
		gl.uniform1f(uniform("uWarpFrequency"), 5);
		gl.uniform1f(uniform("uWarpSpeed"), 2);
		gl.uniform1f(uniform("uWarpAmplitude"), 50);
		gl.uniform1f(uniform("uBlendAngle"), 0);
		gl.uniform1f(uniform("uBlendSoftness"), 0.05);
		gl.uniform1f(uniform("uRotationAmount"), 500);
		gl.uniform1f(uniform("uNoiseScale"), 2);
		gl.uniform1f(uniform("uGrainAmount"), 0.055);
		gl.uniform1f(uniform("uGrainScale"), 2);
		gl.uniform1f(uniform("uGrainAnimated"), 0);
		gl.uniform1f(uniform("uContrast"), 1.25);
		gl.uniform1f(uniform("uGamma"), 1);
		gl.uniform1f(uniform("uSaturation"), 1);
		gl.uniform2f(uniform("uCenterOffset"), 0, 0);
		gl.uniform1f(uniform("uZoom"), 0.9);
		gl.uniform3f(uniform("uColor1"), ...(colors[0] as Rgb));
		gl.uniform3f(uniform("uColor2"), ...(colors[1] as Rgb));
		gl.uniform3f(uniform("uColor3"), ...(colors[2] as Rgb));
		gl.uniform3f(uniform("uColor4"), ...(colors[3] as Rgb));
		gl.uniform1f(uniform("uLightMode"), 1);
		gl.uniform1f(uniforms.brightness, options.dark ? DARK_BRIGHTNESS : 1);

		const draw = () => {
			if (destroyed || !program || !vertexArray) return;
			bindProgram(program);
			gl.bindVertexArray(vertexArray);
			gl.uniform1f(uniforms.time, elapsedSeconds);
			gl.drawArrays(gl.TRIANGLES, 0, 3);
			gl.bindVertexArray(null);
		};

		const resizeAndDraw = (cssWidth: number, cssHeight: number) => {
			const [width, height] = renderSize(
				cssWidth,
				cssHeight,
				ownerWindow.devicePixelRatio,
			);
			if (canvas.width !== width) canvas.width = width;
			if (canvas.height !== height) canvas.height = height;
			gl.viewport(0, 0, width, height);
			bindProgram(program);
			gl.uniform2f(uniforms.resolution, width, height);
			draw();
		};

		const animate = (timestamp: number) => {
			animationFrame = null;
			if (!running || destroyed) return;

			if (lastTickMs === null) {
				lastTickMs = timestamp;
				lastDrawMs = timestamp;
			} else {
				elapsedSeconds += Math.max(0, timestamp - lastTickMs) / 1000;
				lastTickMs = timestamp;
				if (
					lastDrawMs === null ||
					timestamp - lastDrawMs >= FRAME_INTERVAL_MS
				) {
					lastDrawMs = timestamp;
					try {
						draw();
					} catch {
						failAfterReady();
						return;
					}
				}
			}

			animationFrame = requestFrame(animate);
		};

		const initialRect = parent.getBoundingClientRect();
		resizeAndDraw(initialRect.width, initialRect.height);

		resizeObserver = new ResizeObserverConstructor((entries) => {
			if (destroyed) return;
			const entry = entries.find((candidate) => candidate.target === parent);
			if (!entry) return;
			try {
				resizeAndDraw(entry.contentRect.width, entry.contentRect.height);
			} catch {
				failAfterReady();
			}
		});
		resizeObserver.observe(parent);
		canvas.addEventListener("webglcontextlost", handleContextLost);

		return {
			pause() {
				if (destroyed || !running) return;
				running = false;
				lastTickMs = null;
				lastDrawMs = null;
				if (animationFrame !== null) {
					cancelFrame(animationFrame);
					animationFrame = null;
				}
			},
			resume() {
				if (destroyed || running) return;
				running = true;
				lastTickMs = null;
				lastDrawMs = null;
				animationFrame = requestFrame(animate);
			},
			setDarkMode(dark) {
				if (destroyed || !program || !vertexArray) return;
				try {
					bindProgram(program);
					gl.uniform1f(uniforms.brightness, dark ? DARK_BRIGHTNESS : 1);
					draw();
				} catch {
					failAfterReady();
				}
			},
			destroy() {
				destroyInternal();
			},
		};
	} catch {
		releaseResources();
		return null;
	}
}
