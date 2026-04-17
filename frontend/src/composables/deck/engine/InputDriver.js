import { useGraphStore } from '../../../stores/graph'

/**
 * ⌨️ InputDriver — O Piloto de Periféricos
 * 
 * Responsável por capturar entradas de teclado (WASDQE) e 
 * coordenar a movimentação contínua da câmera através do InteractionPilot.
 */
export function useInputDriver() {
  const store = useGraphStore()
  
  const keys = { w: false, a: false, s: false, d: false, q: false, e: false }
  let moveInterval = null
  let fpsRafId = null

  const isInputFocused = () => {
    const el = document.activeElement
    return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
  }

  const startMoving = (currentViewState, panTarget) => {
    const move = () => {
      const vs = currentViewState.value
      if (!vs) return
      
      let dx = 0, dy = 0, dz = 0
      const speed = 25 * Math.max(0.2, (5 - vs.zoom) / 5)

      // Determinação de vetores brutos
      if (keys.w) dz -= speed
      if (keys.s) dz += speed
      if (keys.a) dx -= speed
      if (keys.d) dx += speed
      if (keys.q) dy -= speed
      if (keys.e) dy += speed

      if (dx !== 0 || dy !== 0 || dz !== 0) {
          panTarget(dx, dy, dz) // Delega o cálculo trigonométrico para o Pilot
      }

      moveInterval = requestAnimationFrame(move)
    }
    moveInterval = requestAnimationFrame(move)
  }

  // 📊 FPS Monitor
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
      if (store.showFps) fpsRafId = requestAnimationFrame(measure)
    }
    fpsRafId = requestAnimationFrame(measure)
  }

  const registerKeyboardControls = (currentViewState, panTarget) => {
    const onKeyDown = (e) => {
        if (e.key === 'F1') {
            e.preventDefault()
            store.showFps = !store.showFps
            if (store.showFps) startFpsLoop()
            return
        }

        if (isInputFocused()) return
        const k = e.key.toLowerCase()
        if (k in keys) {
            keys[k] = true
            if (!moveInterval) startMoving(currentViewState, panTarget)
        }
    }

    const onKeyUp = (e) => {
        const k = e.key.toLowerCase()
        if (k in keys) keys[k] = false
        if (!Object.values(keys).some(v => v)) {
            if (moveInterval) {
                cancelAnimationFrame(moveInterval)
                moveInterval = null
            }
        }
    }
    
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)

    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
      if (moveInterval) cancelAnimationFrame(moveInterval)
      if (fpsRafId) cancelAnimationFrame(fpsRafId)
    }
  }

  return { registerKeyboardControls }
}
