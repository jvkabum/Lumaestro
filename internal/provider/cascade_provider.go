package provider

import (
	"context"
	"strings"
	"time"
)

// CascadeProvider orquestra múltiplos geradores de conteúdo em ordem de prioridade.
// É o "Modo Sobrevivência" do Lumaestro.
type CascadeProvider struct {
	Generators []ContentGenerator
	Names      []string
	OnFailover func(from, to, reason string)
}

func NewCascadeProvider(onFailover func(from, to, reason string)) *CascadeProvider {
	return &CascadeProvider{
		Generators: make([]ContentGenerator, 0),
		Names:      make([]string, 0),
		OnFailover: onFailover,
	}
}

func (c *CascadeProvider) Add(name string, gen ContentGenerator) {
	if gen == nil {
		return
	}
	c.Names = append(c.Names, name)
	c.Generators = append(c.Generators, gen)
}

func (c *CascadeProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	var lastErr error

	for i, gen := range c.Generators {
		if gen == nil {
			continue
		}

		res, err := gen.GenerateText(ctx, prompt)
		if err == nil {
			return res, nil
		}

		lastErr = err
		
		// 🛡️ Lógica de Decisão de Fallback
		// Só pulamos para o próximo se for erro de infra/cota.
		// Erros de contexto ou prompt inválido não devem disparar fallback.
		if i < len(c.Generators)-1 && c.isFailoverError(err) {
			if c.OnFailover != nil {
				c.OnFailover(c.Names[i], c.Names[i+1], err.Error())
			}
			// Pequeno cooldown antes de tentar o próximo para estabilizar a rede
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Se chegamos aqui e deu erro, ou não é erro de fallback ou é o último motor
		break
	}

	return "", lastErr
}

func (c *CascadeProvider) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	var lastErr error

	for i, gen := range c.Generators {
		if gen == nil {
			continue
		}

		res, err := gen.GenerateMultimodalText(ctx, prompt, data, mimeType)
		if err == nil {
			return res, nil
		}

		lastErr = err

		if i < len(c.Generators)-1 && c.isFailoverError(err) {
			if c.OnFailover != nil {
				c.OnFailover(c.Names[i], c.Names[i+1], err.Error())
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	return "", lastErr
}

func (c *CascadeProvider) isFailoverError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	
	// Gatilhos comuns de cota/instabilidade
	failoverTriggers := []string{
		"429", "rate limit", "quota", "exhausted", "capacity",
		"503", "unavailable", "timeout", "network", "connection",
		"forbidden", "403", "unauthorized", "api key", "400", "context length",
	}

	for _, trigger := range failoverTriggers {
		if strings.Contains(msg, trigger) {
			return true
		}
	}
	return false
}

func (c *CascadeProvider) Stop() {
	for _, gen := range c.Generators {
		if gen == nil {
			continue
		}
		// Tenta encerramento seguro se o motor tiver o método Stop
		if stoppable, ok := gen.(interface{ Stop() }); ok {
			stoppable.Stop()
		}
	}
}
