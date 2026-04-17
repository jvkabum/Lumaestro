import { FlyToInterpolator } from '@deck.gl/core';

/**
 * 🎮 InteractionPilot — O Navegador Espacial
 * 
 * Responsável pelo controle de voo cinematográfico, 
 * recentralização do universo e ponte WASDQE.
 */
export function useInteractionPilot() {
    
    // Voar até um nó específico
    const focusNode = (deckInstance, currentViewState, node) => {
        if (!deckInstance || !node) return;

        const targetViewState = {
            ...currentViewState.value,
            target: [node.x, node.y, node.z],
            distance: 300,
            transitionDuration: 2500,
            transitionInterpolator: new FlyToInterpolator({ speed: 1.2 })
        };

        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    // Centralizar visão
    const zoomToFit = (deckInstance, currentViewState) => {
        if (!deckInstance) return;
        const targetViewState = {
            ...currentViewState.value,
            target: [0, 0, 0],
            zoom: -3.2,
            rotationX: 30,
            rotationOrbit: -25,
            transitionDuration: 1500,
            transitionInterpolator: new FlyToInterpolator()
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    // Movimentação WASDQE
    const panTarget = (deckInstance, currentViewState, moveRawX, moveRawY, moveRawZ) => {
        if (!deckInstance) return;
        
        const yaw = currentViewState.value.rotationOrbit * (Math.PI / 180);
        const dx = moveRawX * Math.cos(yaw) + moveRawZ * Math.sin(yaw);
        const dz = -moveRawX * Math.sin(yaw) + moveRawZ * Math.cos(yaw);

        const speedFactor = Math.max(0.5, 1 / (currentViewState.value.zoom || 0.1)) * 0.1;

        const tX = currentViewState.value.target[0] + dx * speedFactor;
        const tY = currentViewState.value.target[1] + moveRawY * speedFactor;
        const tZ = currentViewState.value.target[2] + dz * speedFactor;

        const targetViewState = {
            ...currentViewState.value,
            target: [tX, tY, tZ],
            transitionDuration: 0
        };
        
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    return { focusNode, zoomToFit, panTarget };
}
