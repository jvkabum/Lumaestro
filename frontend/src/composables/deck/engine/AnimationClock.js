/**
 * ⏱️ AnimationClock — O Relógio do Cosmos
 * 
 * Gerencia o loop de animação (RequestAnimationFrame) e o tempo 
 * global para efeitos visuais pulsantes (fótons).
 */
export function useAnimationClock() {
    let animationRafId = null;
    let animationTime = 0;

    const startClock = (onTick) => {
        const march = () => {
            animationTime += 0.007;
            if (animationTime >= 100000) animationTime = 0;
            
            if (onTick) onTick(animationTime);
            
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

    return { startClock, stopClock, getTick };
}
