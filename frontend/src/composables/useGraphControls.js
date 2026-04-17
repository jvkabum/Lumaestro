import { useGraphStore } from '../stores/graph'
import { toRaw } from 'vue'

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

  const startMoving = (viewStateRef) => {
    const move = () => {
      const vs = viewStateRef.value
      if (!vs) return
      
      const speed = 25 * Math.max(0.2, (5 - vs.zoom) / 5) // Velocidade adaptativa ao zoom
      const bearingRad = (vs.bearing * Math.PI) / 180
      
      // Vetores Direcionais Relativos à Câmera
      const sinB = Math.sin(bearingRad)
      const cosB = Math.cos(bearingRad)
      
      let dx = 0, dy = 0, dz = 0

      // W/S: Frente/Trás (Baseado no Bearing)
      if (keys.w) { dx += sinB * speed; dz -= cosB * speed; }
      if (keys.s) { dx -= sinB * speed; dz += cosB * speed; }
      
      // A/D: Strafe (Lateral)
      if (keys.a) { dx -= cosB * speed; dz -= sinB * speed; }
      if (keys.d) { dx += cosB * speed; dz += sinB * speed; }
      
      // Q/E: Vertical (Eixo Y)
      if (keys.q) dy -= speed
      if (keys.e) dy += speed

      if (dx !== 0 || dy !== 0 || dz !== 0) {
        // Atualiza o target do Deck.gl diretamente
        const currentTarget = vs.target || [0,0,0]
        viewStateRef.value = {
          ...vs,
          target: [
            currentTarget[0] + dx,
            currentTarget[1] + dy,
            currentTarget[2] + dz
          ],
          transitionDuration: 0 // Movimento instantâneo para fluidez total
        }
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

  const handleKeyDown = (e, viewStateRef) => {
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
      if (!moveInterval) startMoving(viewStateRef)
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
  const registerKeyboardControls = (viewStateRef) => {
    const onKeyDown = (e) => handleKeyDown(e, viewStateRef)
    const onKeyUp = (e) => handleKeyUp(e)
    
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)

    // Retorna a função de limpeza
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
      if (moveInterval) cancelAnimationFrame(moveInterval)
      stopFpsLoop()
    }
  }

  return { registerKeyboardControls }
}
