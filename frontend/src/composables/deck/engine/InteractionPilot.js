import { LinearInterpolator } from '@deck.gl/core';

/**
 * 🎮 InteractionPilot — O Navegador Espacial (V2 Premium)
 * 
 * Responsável pelo controle de voo cinematográfico, 
 * transições de câmera, binds WASDQE e Presets.
 */
export function useInteractionPilot() {

    // 🎛️ CONFIGURAÇÃO DE NAVEGAÇÃO
    const NAV_CONFIG = {
        minZoom: -15, maxZoom: 50,
        minPitch: -Infinity, maxPitch: Infinity, // Sem limites, pode virar de cabeça pra baixo
        transitionInterpolator: new LinearInterpolator(['rotationX', 'rotationOrbit', 'zoom', 'target'])
    };

    // 🎯 View Presets — Atalhos para ângulos úteis
    const VIEW_PRESETS = {
        top: { pitch: 0, bearing: 0, zoom: 10 },
        isometric: { pitch: 45, bearing: 45, zoom: 10 },
        side: { pitch: 0, bearing: 90, zoom: 10 },
        close: { pitch: 30, bearing: 0, zoom: 15 },
        overview: { pitch: 20, bearing: 0, zoom: 5 }
    };

    // --- LEGACO (MANTIDO P/ STORECONTRACT E UI CLÁSSICA) ---
    const focusNode = (deckInstance, currentViewState, node) => {
        if (!deckInstance || !node) return;
        const targetViewState = {
            ...currentViewState.value,
            target: [node.x, node.y, node.z],
            zoom: 0.5, // Reduzido de 3.5 para 1.5 para a câmera parar mais longe do nó
            transitionDuration: 3500, // Efeito "nave viajando"
            transitionInterpolator: NAV_CONFIG.transitionInterpolator
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    const zoomToFit = (deckInstance, currentViewState) => {
        if (!deckInstance) return;
        const targetViewState = {
            ...currentViewState.value,
            target: [0, 0, 0], zoom: -3.2, rotationX: 30, rotationOrbit: -25,
            transitionDuration: 1500,
            transitionInterpolator: NAV_CONFIG.transitionInterpolator
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    const panTarget = (deckInstance, currentViewState, moveRawX, moveRawY, moveRawZ) => {
        if (!deckInstance) return;
        const yaw = currentViewState.value.rotationOrbit * (Math.PI / 180);
        const dx = moveRawX * Math.cos(yaw) + moveRawZ * Math.sin(yaw);
        const dz = -moveRawX * Math.sin(yaw) + moveRawZ * Math.cos(yaw);
        const speedFactor = Math.max(0.5, 1 / (currentViewState.value.zoom || 0.1)) * 0.1;

        const targetViewState = {
            ...currentViewState.value,
            target: [currentViewState.value.target[0] + dx * speedFactor, currentViewState.value.target[1] + moveRawY * speedFactor, currentViewState.value.target[2] + dz * speedFactor],
            transitionDuration: 0
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    // --- FERRAMENTAS V2 (PREMIUM NAV) ---
    const goToPreset = (deckInstance, currentViewState, presetName, options = {}) => {
        const preset = VIEW_PRESETS[presetName];
        if (!preset || !deckInstance) return;

        const targetViewState = {
            ...currentViewState.value,
            ...preset,
            ...options,
            transitionDuration: NAV_CONFIG.transitionDuration,
            transitionInterpolator: NAV_CONFIG.transitionInterpolator
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    const animateTo = (deckInstance, currentViewState, targetProps, duration = 1000) => {
        if (!deckInstance) return;
        const targetViewState = {
            ...currentViewState.value,
            ...targetProps,
            transitionDuration: duration,
            transitionInterpolator: NAV_CONFIG.transitionInterpolator
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    const setupKeyboardNav = (deckInstance, currentViewState, containerRef, store, speed = 10) => {
        if (!deckInstance || !containerRef) return () => { };

        const keys = {};
        let animationFrame;

        const handleKey = (e) => {
            if (['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) return;

            keys[e.key.toLowerCase()] = e.type === 'keydown';

            if (e.shiftKey !== undefined) keys['shift'] = e.shiftKey;
            if (e.ctrlKey !== undefined) keys['control'] = e.ctrlKey;

            if (e.type === 'keydown') {
                if (e.key === '1') goToPreset(deckInstance, currentViewState, 'top');
                if (e.key === '2') goToPreset(deckInstance, currentViewState, 'isometric');
                if (e.key === '3') goToPreset(deckInstance, currentViewState, 'side');
            }
        };

        const moveLoop = () => {
            const view = currentViewState.value;
            let changed = false;
            let newView = { ...view, target: [...(view.target || [0, 0, 0])] };

            // Shift e Ctrl - Zoom
            if (keys['shift']) { newView.zoom = Math.min(NAV_CONFIG.maxZoom, (newView.zoom || 0) + 0.08); changed = true; }
            if (keys['control']) { newView.zoom = Math.max(NAV_CONFIG.minZoom, (newView.zoom || 0) - 0.08); changed = true; }

            // A e D - Rotação horizontal (Orbit)
            if (keys['a']) { newView.rotationOrbit = (newView.rotationOrbit || 0) - 2.5; changed = true; }
            if (keys['d']) { newView.rotationOrbit = (newView.rotationOrbit || 0) + 2.5; changed = true; }

            // W e S - Inclinação vertical (respeitada pelo controller com minRotationX:-179)
            if (keys['w']) { newView.rotationX = (newView.rotationX || 0) + 2.5; changed = true; }
            if (keys['s']) { newView.rotationX = (newView.rotationX || 0) - 2.5; changed = true; }

            // Q e E - Translação Y
            if (keys['e']) { newView.target[1] += speed * 2; changed = true; }
            if (keys['q']) { newView.target[1] -= speed * 2; changed = true; }

            // Espaço - Reset
            if (keys[' ']) {
                keys[' '] = false;
                changed = false;
                zoomToFit(deckInstance, currentViewState);
            }

            // Esc - Labels
            if (keys['escape']) {
                keys['escape'] = false;
                store.showLabels = store.showLabels === undefined ? false : !store.showLabels;
                changed = true;
            }

            if (changed) {
                newView.zoom = Math.max(NAV_CONFIG.minZoom, Math.min(NAV_CONFIG.maxZoom, newView.zoom));
                deckInstance.setProps({ initialViewState: newView });
                currentViewState.value = newView;
            }

            animationFrame = requestAnimationFrame(moveLoop);
        };

        containerRef.addEventListener('keydown', handleKey);
        containerRef.addEventListener('keyup', handleKey);
        moveLoop();

        return () => {
            containerRef?.removeEventListener('keydown', handleKey);
            containerRef?.removeEventListener('keyup', handleKey);
            if (animationFrame) cancelAnimationFrame(animationFrame);
        };
    };

    return {
        focusNode, zoomToFit, panTarget,
        goToPreset, animateTo, setupKeyboardNav,
        VIEW_PRESETS, NAV_CONFIG
    };
}
