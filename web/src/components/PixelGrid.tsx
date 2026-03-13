import { useEffect, useRef } from 'react';
import * as THREE from 'three';

const MAX_CLICKS = 10;

const vertexShader = `
  void main() {
    gl_Position = vec4(position, 1.0);
  }
`;

const fragmentShader = `
  precision highp float;

  uniform float iTime;
  uniform vec2 iResolution;
  uniform vec2 clickPositions[${MAX_CLICKS}];
  uniform float clickTimes[${MAX_CLICKS}];
  uniform int clickCount;
  uniform vec2 invaderCenter;
  uniform float invaderRadius;
  uniform vec2 mousePos;
  uniform float mouseActive;

  // --- Simplex 3D noise ---
  vec4 permute(vec4 x) { return mod(((x*34.0)+1.0)*x, 289.0); }
  vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }

  float snoise(vec3 v) {
    const vec2 C = vec2(1.0/6.0, 1.0/3.0);
    const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);
    vec3 i = floor(v + dot(v, C.yyy));
    vec3 x0 = v - i + dot(i, C.xxx);
    vec3 g = step(x0.yzx, x0.xyz);
    vec3 l = 1.0 - g;
    vec3 i1 = min(g.xyz, l.zxy);
    vec3 i2 = max(g.xyz, l.zxy);
    vec3 x1 = x0 - i1 + C.xxx;
    vec3 x2 = x0 - i2 + C.yyy;
    vec3 x3 = x0 - D.yyy;
    i = mod(i, 289.0);
    vec4 p = permute(permute(permute(
      i.z + vec4(0.0, i1.z, i2.z, 1.0))
      + i.y + vec4(0.0, i1.y, i2.y, 1.0))
      + i.x + vec4(0.0, i1.x, i2.x, 1.0));
    float n_ = 1.0/7.0;
    vec3 ns = n_ * D.wyz - D.xzx;
    vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
    vec4 x_ = floor(j * ns.z);
    vec4 y_ = floor(j - 7.0 * x_);
    vec4 x = x_ * ns.x + ns.yyyy;
    vec4 y = y_ * ns.x + ns.yyyy;
    vec4 h = 1.0 - abs(x) - abs(y);
    vec4 b0 = vec4(x.xy, y.xy);
    vec4 b1 = vec4(x.zw, y.zw);
    vec4 s0 = floor(b0)*2.0 + 1.0;
    vec4 s1 = floor(b1)*2.0 + 1.0;
    vec4 sh = -step(h, vec4(0.0));
    vec4 a0 = b0.xzyw + s0.xzyw*sh.xxyy;
    vec4 a1 = b1.xzyw + s1.xzyw*sh.zzww;
    vec3 p0 = vec3(a0.xy, h.x);
    vec3 p1 = vec3(a0.zw, h.y);
    vec3 p2 = vec3(a1.xy, h.z);
    vec3 p3 = vec3(a1.zw, h.w);
    vec4 norm = taylorInvSqrt(vec4(dot(p0,p0),dot(p1,p1),dot(p2,p2),dot(p3,p3)));
    p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
    vec4 m = max(0.6 - vec4(dot(x0,x0),dot(x1,x1),dot(x2,x2),dot(x3,x3)), 0.0);
    m = m * m;
    return 42.0 * dot(m*m, vec4(dot(p0,x0),dot(p1,x1),dot(p2,x2),dot(p3,x3)));
  }

  // --- fBm (6 octaves) ---
  float fbm(vec3 p) {
    float value = 0.0;
    float amplitude = 0.5;
    float frequency = 1.0;
    for (int i = 0; i < 6; i++) {
      value += amplitude * snoise(p * frequency);
      frequency *= 2.0;
      amplitude *= 0.5;
    }
    return value;
  }

  // --- Bayer 8x8 dithering ---
  float bayer8(vec2 pos) {
    int x = int(mod(pos.x, 8.0));
    int y = int(mod(pos.y, 8.0));
    int index = x + y * 8;
    float b[64];
    b[0]=0.0;b[1]=32.0;b[2]=8.0;b[3]=40.0;b[4]=2.0;b[5]=34.0;b[6]=10.0;b[7]=42.0;
    b[8]=48.0;b[9]=16.0;b[10]=56.0;b[11]=24.0;b[12]=50.0;b[13]=18.0;b[14]=58.0;b[15]=26.0;
    b[16]=12.0;b[17]=44.0;b[18]=4.0;b[19]=36.0;b[20]=14.0;b[21]=46.0;b[22]=6.0;b[23]=38.0;
    b[24]=60.0;b[25]=28.0;b[26]=52.0;b[27]=20.0;b[28]=62.0;b[29]=30.0;b[30]=54.0;b[31]=22.0;
    b[32]=3.0;b[33]=35.0;b[34]=11.0;b[35]=43.0;b[36]=1.0;b[37]=33.0;b[38]=9.0;b[39]=41.0;
    b[40]=51.0;b[41]=19.0;b[42]=59.0;b[43]=27.0;b[44]=49.0;b[45]=17.0;b[46]=57.0;b[47]=25.0;
    b[48]=15.0;b[49]=47.0;b[50]=7.0;b[51]=39.0;b[52]=13.0;b[53]=45.0;b[54]=5.0;b[55]=37.0;
    b[56]=63.0;b[57]=31.0;b[58]=55.0;b[59]=23.0;b[60]=61.0;b[61]=29.0;b[62]=53.0;b[63]=21.0;
    float val = 0.0;
    for (int i = 0; i < 64; i++) {
      if (i == index) { val = b[i]; break; }
    }
    return val / 64.0;
  }

  void main() {
    float t = iTime * 0.5;

    // Pixelate to 8x8 grid
    float pixelSize = 8.0;
    vec2 pixelUV = floor(gl_FragCoord.xy / pixelSize) * pixelSize;
    vec2 pUV = pixelUV / iResolution.xy;

    // --- Invader force field ---
    float aspect = iResolution.x / iResolution.y;
    vec2 invUV = invaderCenter / iResolution.xy;
    vec2 toPixel = pUV - invUV;
    vec2 toPixelAspect = toPixel * vec2(aspect, 1.0);
    float invDist = length(toPixelAspect);
    float radiusUV = invaderRadius / iResolution.y;

    // Push noise sampling coords away from invader (organic flow-around)
    float repulsion = smoothstep(radiusUV * 1.3, 0.0, invDist);
    vec2 pushDir = normalize(toPixel + vec2(0.0001));
    vec2 noisePUV = pUV + pushDir * repulsion * 0.25;

    // Smooth fade near invader center
    float invaderFade = smoothstep(radiusUV * 0.35, radiusUV * 1.1, invDist);

    // --- Cursor cluster ---
    vec2 mouseUV = mousePos / iResolution.xy;
    vec2 toMouse = pUV - mouseUV;
    vec2 toMouseAspect = toMouse * vec2(aspect, 1.0);
    float mouseDist = length(toMouseAspect);

    // Irregular circle edge via angular noise (two octaves for organic wobble)
    float mouseAngle = atan(toMouseAspect.y, toMouseAspect.x);
    float edgeWobble = snoise(vec3(mouseAngle * 3.0, iTime * 1.5, 7.0)) * 0.025
                     + snoise(vec3(mouseAngle * 7.0, iTime * 0.8, 13.0)) * 0.012;
    float cursorR = 0.12 + edgeWobble;

    // --- Click explosion: scatter pixels outward from click, then reassemble ---
    vec2 scatterOffset = vec2(0.0);
    float scatterIntensity = 0.0;
    for (int i = 0; i < ${MAX_CLICKS}; i++) {
      if (i >= clickCount) break;
      float elapsed = iTime - clickTimes[i];
      if (elapsed < 0.0 || elapsed > 2.5) continue;

      vec2 clickUV = clickPositions[i] / iResolution.xy;
      vec2 toClick = pUV - clickUV;
      vec2 toClickAspect = toClick * vec2(aspect, 1.0);
      float clickDist = length(toClickAspect);

      // Only scatter pixels that were in the cursor cluster area
      float inCluster = smoothstep(cursorR * 3.5, 0.0, clickDist);
      if (inCluster > 0.01) {
        // Fast burst out (0–0.2s), slow drift back (0.2–2.5s)
        float phase;
        if (elapsed < 0.2) {
          phase = elapsed / 0.2;
        } else {
          phase = 1.0 - (elapsed - 0.2) / 2.3;
        }
        phase = clamp(phase, 0.0, 1.0);
        float eased = 1.0 - pow(1.0 - phase, 3.0);

        float scatter = eased * inCluster;

        // Radial push direction from click center
        vec2 dir = normalize(toClick + vec2(0.0001));
        scatterOffset += dir * scatter * 0.12;
        scatterIntensity = max(scatterIntensity, scatter);
      }
    }

    // Apply scatter displacement to noise sampling (pixels visually fly outward)
    noisePUV += scatterOffset;

    // Noise coordinates with turbulent flow
    float flowX = sin(t * 0.2) * 3.0 + cos(t * 0.15) * 2.0;
    float flowY = cos(t * 0.18) * 2.5 + sin(t * 0.12) * 1.5;
    vec3 noiseCoord = vec3(noisePUV * 4.0 + vec2(flowX, flowY), t * 0.3);

    // fBm noise for organic clouds
    float n = fbm(noiseCoord);

    // Second noise for color selection (purple vs green)
    float colorNoise = snoise(vec3(noisePUV * 3.0 + vec2(t * 0.1), t * 0.05 + 100.0));

    // Click wave accumulation (global ripple across entire grid)
    float waveEffect = 0.0;
    for (int i = 0; i < ${MAX_CLICKS}; i++) {
      if (i >= clickCount) break;
      float elapsed = iTime - clickTimes[i];
      if (elapsed < 0.0 || elapsed > 3.5) continue;

      vec2 clickUV = clickPositions[i] / iResolution.xy;
      float clickDist = distance(pUV, clickUV) * aspect;
      float wave = sin(clickDist * 15.0 - elapsed * 3.0);
      float spatial = exp(-clickDist * 3.5);
      float temporal = smoothstep(3.5, 0.0, elapsed);
      wave = pow(max(wave, 0.0), 2.5) * spatial * temporal * 0.45;
      waveEffect += wave;
    }

    // --- Cursor gather: boost pixel brightness near mouse ---
    float gatherFade = smoothstep(cursorR, cursorR * 0.1, mouseDist);
    // Suppress gather during explosion (center empties), keep scattered pixels visible
    float gather = gatherFade * mouseActive * (1.0 - scatterIntensity * 0.9);
    float scatterGlow = scatterIntensity * mouseActive * 0.35;

    // Combine noise + waves + cursor effects
    float value = n + waveEffect + gather * 0.55 + scatterGlow;

    // Brightness & contrast — darker = fewer visible pixels
    value = (value - 0.75) * 1.4 + 0.5;

    // Bayer dithering threshold
    float threshold = bayer8(pixelUV / pixelSize) * 0.5;
    float pixel = step(threshold, value * 0.5);

    // Apply invader force field — pixels organically avoid the invader
    pixel *= invaderFade;

    // Color: purple primary, green accent based on noise
    vec3 purple = vec3(0.545, 0.361, 0.965);  // #8b5cf6
    vec3 green = vec3(0.290, 0.867, 0.498);   // #4ade80
    vec3 color = mix(purple, green, smoothstep(0.3, 0.7, colorNoise * 0.5 + 0.5));

    gl_FragColor = vec4(color * pixel, pixel * 0.5);
  }
`;

interface PixelGridProps {
  heroRef?: React.RefObject<HTMLDivElement | null>;
}

export function PixelGrid({ heroRef }: PixelGridProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<{
    renderer: THREE.WebGLRenderer;
    scene: THREE.Scene;
    camera: THREE.OrthographicCamera;
    material: THREE.ShaderMaterial;
    animId: number;
  } | null>(null);
  const clicksRef = useRef<{ x: number; y: number; time: number }[]>([]);
  const startTimeRef = useRef(0);
  const mouseRef = useRef({ x: 0, y: 0, smoothX: 0, smoothY: 0, active: 0, smoothActive: 0 });

  // Initialize start time lazily in effect to avoid impure call during render
  useEffect(() => {
    startTimeRef.current = performance.now() / 1000;
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

    const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: false });
    renderer.setClearColor(0x000000, 0);
    container.appendChild(renderer.domElement);
    renderer.domElement.style.cssText = 'position:absolute;inset:0;pointer-events:none;z-index:1;border-radius:inherit;';

    const scene = new THREE.Scene();
    const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);

    const clickPositions = new Float32Array(MAX_CLICKS * 2);
    const clickTimes = new Float32Array(MAX_CLICKS);

    const material = new THREE.ShaderMaterial({
      vertexShader,
      fragmentShader,
      transparent: true,
      uniforms: {
        iTime: { value: 0 },
        iResolution: { value: new THREE.Vector2() },
        clickPositions: { value: clickPositions },
        clickTimes: { value: clickTimes },
        clickCount: { value: 0 },
        invaderCenter: { value: new THREE.Vector2() },
        invaderRadius: { value: 0 },
        mousePos: { value: new THREE.Vector2() },
        mouseActive: { value: 0 },
      },
    });

    const geometry = new THREE.PlaneGeometry(2, 2);
    scene.add(new THREE.Mesh(geometry, material));

    let currentDpr = Math.min(window.devicePixelRatio, 2);

    const resize = () => {
      const rect = container.getBoundingClientRect();
      currentDpr = Math.min(window.devicePixelRatio, 2);
      renderer.setSize(rect.width, rect.height);
      renderer.setPixelRatio(currentDpr);
      material.uniforms.iResolution.value.set(rect.width * currentDpr, rect.height * currentDpr);

      // Update invader force field position (visible only on lg+ screens)
      const isLg = window.innerWidth >= 1024;
      if (isLg) {
        // Invader: absolute right-8 top-1/2 -translate-y-1/2, size="md" (~180px wide)
        const invaderApproxWidth = 180;
        const cssX = rect.width - 32 - invaderApproxWidth / 2;
        const cssY = rect.height / 2;
        // GL coords: x same direction, y flipped (0 = bottom)
        material.uniforms.invaderCenter.value.set(cssX * currentDpr, (rect.height - cssY) * currentDpr);
        material.uniforms.invaderRadius.value = 140 * currentDpr;
      } else {
        material.uniforms.invaderRadius.value = 0;
      }
    };
    resize();

    const ro = new ResizeObserver(resize);
    ro.observe(container);

    const ref = { renderer, scene, camera, material, animId: 0 };
    sceneRef.current = ref;

    const mouse = mouseRef.current;

    const animate = () => {
      const now = performance.now() / 1000 - startTimeRef.current;
      material.uniforms.iTime.value = now;

      // Smooth mouse position and active state (lerp each frame)
      mouse.smoothX += (mouse.x - mouse.smoothX) * 0.4;
      mouse.smoothY += (mouse.y - mouse.smoothY) * 0.4;
      mouse.smoothActive += (mouse.active - mouse.smoothActive) * 0.12;
      material.uniforms.mousePos.value.set(mouse.smoothX, mouse.smoothY);
      material.uniforms.mouseActive.value = mouse.smoothActive;

      // Update click uniforms
      const clicks = clicksRef.current;
      material.uniforms.clickCount.value = clicks.length;
      for (let i = 0; i < MAX_CLICKS; i++) {
        if (i < clicks.length) {
          clickPositions[i * 2] = clicks[i].x;
          clickPositions[i * 2 + 1] = clicks[i].y;
          clickTimes[i] = clicks[i].time;
        }
      }

      // Remove expired clicks
      clicksRef.current = clicks.filter(c => now - c.time < 3.5);

      renderer.render(scene, camera);
      ref.animId = requestAnimationFrame(animate);
    };
    ref.animId = requestAnimationFrame(animate);

    // Listen for clicks on the hero section parent (captures clicks everywhere,
    // including over text, buttons, and the invader — events bubble up naturally)
    const clickTarget = heroRef?.current || container;
    const onClick = (e: MouseEvent) => {
      const rect = container.getBoundingClientRect();
      const x = (e.clientX - rect.left) * currentDpr;
      const y = (rect.height - (e.clientY - rect.top)) * currentDpr; // flip Y for GL
      const now = performance.now() / 1000 - startTimeRef.current;

      if (clicksRef.current.length >= MAX_CLICKS) {
        clicksRef.current.shift();
      }
      clicksRef.current.push({ x, y, time: now });
    };
    clickTarget.addEventListener('click', onClick);

    // Mouse tracking for cursor cluster
    const onMouseMove = (e: MouseEvent) => {
      const rect = container.getBoundingClientRect();
      mouse.x = (e.clientX - rect.left) * currentDpr;
      mouse.y = (rect.height - (e.clientY - rect.top)) * currentDpr;
      mouse.active = 1;
    };
    const onMouseLeave = () => {
      mouse.active = 0;
    };
    clickTarget.addEventListener('mousemove', onMouseMove);
    clickTarget.addEventListener('mouseleave', onMouseLeave);

    return () => {
      cancelAnimationFrame(ref.animId);
      ro.disconnect();
      clickTarget.removeEventListener('click', onClick);
      clickTarget.removeEventListener('mousemove', onMouseMove);
      clickTarget.removeEventListener('mouseleave', onMouseLeave);
      renderer.dispose();
      geometry.dispose();
      material.dispose();
      if (renderer.domElement.parentNode) {
        renderer.domElement.parentNode.removeChild(renderer.domElement);
      }
    };
  }, [heroRef]);

  return (
    <div
      ref={containerRef}
      style={{
        position: 'absolute',
        inset: 0,
        pointerEvents: 'none',
        zIndex: 1,
        borderRadius: 'inherit',
        overflow: 'hidden',
      }}
    />
  );
}
