/**
 * ⏱️ AnimationClock — O Relógio do Cosmos
 * 
 * Gerencia o loop de animação (RequestAnimationFrame) e o tempo 
 * global para efeitos visuais pulsantes (fótons), incluindo cálculo de FPS.
 */
export function useAnimationClock() {
    let animationRafId = null;
    let animationTime = 0;
    
    // 📊 Telemetria de Performance
    let lastTime = 0;
    let fps = 60;
    const fpsSamples = [];
    const MAX_SAMPLES = 60;

    const startClock = (onTick) => {
        lastTime = performance.now();
        
        const march = (now) => {
            // 🕒 Cálculo de Delta Time
            const dt = (now - lastTime) / 1000;
            lastTime = now;
            
            // 📈 Cálculo de FPS com Média Móvel
            if (dt > 0) {
                const currentFps = 1 / dt;
                fpsSamples.push(currentFps);
                if (fpsSamples.length > MAX_SAMPLES) fpsSamples.shift();
                fps = Math.round(fpsSamples.reduce((a, b) => a + b) / fpsSamples.length);
            }

            // 🧬 Tempo Global de Animação (Fótons)
            animationTime += 0.007;
            if (animationTime >= 100000) animationTime = 0;
            
            if (onTick) onTick(animationTime, dt);
            
            animationRafId = requestAnimationFrame(march);
        };
        
        stopClock();
        animationRafId = requestAnimationFrame(march);
    };

    const stopClock = () => {
        if (animationRafId) {
            cancelAnimationFrame(animationRafId);
            animationRafId = null;
        }
    };

    const getTick = () => animationTime;
    const getFPS = () => fps;

    return { startClock, stopClock, getTick, getFPS };
}

