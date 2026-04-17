import { FlyToInterpolator, LinearInterpolator } from '@deck.gl/core';

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
            distance: 300,
            transitionDuration: 1500,
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
        if (!deckInstance || !containerRef) return () => {};
        
        const keys = {};
        let animationFrame;
        
        const handleKey = (e) => {
            if (['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) return;
            
            // Grava a tecla principal
            keys[e.key.toLowerCase()] = e.type === 'keydown';
            
            // Grava o estado físico explícito dos modificadores na malha do OS
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
            let newView = { ...view, target: [...(view.target || [0,0,0])] };
            
            // Novo Paradigma de Controles "Galáxia como uma Bola Fixa"
            
            // Shift e Ctrl - Controle de Zoom Estelar (Ir para Frente / Voltar)
            if (keys['shift']) { newView.zoom = Math.min(NAV_CONFIG.maxZoom, (newView.zoom || 0) + 0.08); changed = true; }
            if (keys['control']) { newView.zoom = Math.max(NAV_CONFIG.minZoom, (newView.zoom || 0) - 0.08); changed = true; }

            // A e D - Gira a Galáxia no Eixo Estacionário (Rotação Horizontal / Orbit)
            if (keys['a']) { newView.rotationOrbit = (newView.rotationOrbit || newView.bearing || 0) - 2.5; changed = true; }
            if (keys['d']) { newView.rotationOrbit = (newView.rotationOrbit || newView.bearing || 0) + 2.5; changed = true; }
            
            // W e S - Inclina a Galáxia para cima e para baixo (Roll / Pitch contínuo sem trava)
            if (keys['w']) { newView.rotationX = (newView.rotationX || newView.pitch || 30) + 2.5; changed = true; }
            if (keys['s']) { newView.rotationX = (newView.rotationX || newView.pitch || 30) - 2.5; changed = true; }
            
            // Q e E - Subir e Descer a Galáxia (Translação do Eixo Y)
            if (keys['e']) { 
                view.target = view.target || [0,0,0];
                newView.target[1] += speed * 2; 
                changed = true; 
            }
            if (keys['q']) { 
                view.target = view.target || [0,0,0];
                newView.target[1] -= speed * 2; 
                changed = true; 
            }

            // Espaço - Ponto de Fuga (Recentrar Galáxia Animado)
            if (keys[' ']) {
                keys[' '] = false; // Consome a tecla instantaneamente para não engasgar o frame
                changed = false; // Aborta a atualização manual padrão deste frame
                zoomToFit(deckInstance, currentViewState); // Deixa a função injetar a ViewState de Vôo
            }

            // Esc - Alternar Texto/Títulos Visuais na tela
            if (keys['escape']) {
                 keys['escape'] = false; // Debounce manual do loop
                 store.showLabels = store.showLabels === undefined ? false : !store.showLabels;
                 changed = true; // Força uma leve perturbação para garantir Render rápido
            }
            
            if (changed) {
                newView.rotationX = newView.rotationX; // Infinito sem trava
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
