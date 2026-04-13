import * as THREE from 'three'
import { useGraphStore } from '../stores/graph'

/**
 * 🎮 useGraphControls — Navegação Gamificada (WASD + QE)
 * 
 * Responsável por:
 * - Captura de teclas WASD + Q/E para movimentação FPS
 * - Cálculo de vetores de direção via câmera THREE.js
 * - Loop de animação via requestAnimationFrame
 * - Proteção contra input em campos de texto
 */
export function useGraphControls() {
  const store = useGraphStore()
  
  const keys = { w: false, a: false, s: false, d: false, q: false, e: false }
  const moveSpeed = 20
  let moveInterval = null
  let fpsRafId = null

  const isInputFocused = () => {
    const el = document.activeElement
    return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
  }

  const startMoving = () => {
    const move = () => {
      const Graph = store.graphInstance
      if (!Graph || typeof Graph.panTarget !== 'function') return
      
      let dx = 0, dy = 0, dz = 0

      // Intenções Brutas de Movimento
      if (keys.w) dz -= moveSpeed
      if (keys.s) dz += moveSpeed
      if (keys.a) dx -= moveSpeed
      if (keys.d) dx += moveSpeed
      if (keys.q) dy -= moveSpeed
      if (keys.e) dy += moveSpeed

      if (dx !== 0 || dy !== 0 || dz !== 0) {
         // Delega o cálculo do quaternion/yaw para o Motor Gráfico Ativo (Deck.gl)
         Graph.panTarget(dx, dy, dz)
      }

      moveInterval = requestAnimationFrame(move)
    }
    moveInterval = requestAnimationFrame(move)
  }

  // 📊 FPS Monitor — Loop de medição real via rAF
  const startFpsLoop = () => {
    let frameCount = 0
    let lastTime = performance.now()

    const measure = () => {
      frameCount++
      const now = performance.now()
      const elapsed = now - lastTime
      if (elapsed >= 1000) {
        store.currentFps = Math.round((frameCount * 1000) / elapsed)
        frameCount = 0
        lastTime = now
      }
      if (store.showFps) {
        fpsRafId = requestAnimationFrame(measure)
      } else {
        fpsRafId = null
      }
    }
    fpsRafId = requestAnimationFrame(measure)
  }

  const stopFpsLoop = () => {
    if (fpsRafId) {
      cancelAnimationFrame(fpsRafId)
      fpsRafId = null
    }
  }

  const handleKeyDown = (e) => {
    // F1: Toggle FPS counter
    if (e.key === 'F1') {
      e.preventDefault()
      store.showFps = !store.showFps
      if (store.showFps) {
        startFpsLoop()
      } else {
        stopFpsLoop()
      }
      return
    }

    if (isInputFocused()) return
    const k = e.key.toLowerCase()
    if (k in keys) {
      keys[k] = true
      if (!moveInterval) startMoving()
    }
  }

  const handleKeyUp = (e) => {
    const k = e.key.toLowerCase()
    if (k in keys) keys[k] = false
    
    // Para o loop se todas as teclas forem soltas
    if (!Object.values(keys).some(v => v)) {
      if (moveInterval) {
        cancelAnimationFrame(moveInterval)
        moveInterval = null
      }
    }
  }

  /**
   * Registra os listeners de teclado no window
   * @returns {Function} Cleanup function para chamar em onUnmounted
   */
  const registerKeyboardControls = () => {
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)

    // Retorna a função de limpeza
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
      if (moveInterval) cancelAnimationFrame(moveInterval)
      stopFpsLoop()
    }
  }

  return { registerKeyboardControls }
}
