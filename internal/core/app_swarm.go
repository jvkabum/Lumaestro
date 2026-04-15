package core

import (
	"Lumaestro/internal/db"
	"Lumaestro/internal/orchestration"
	"Lumaestro/internal/prompts"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ============================================================
// ORQUESTRAÇÃO SWARM (PAPERCLIP MODE)
// ============================================================

func (a *App) startOrchestration() {
	orchestration.StartHeartbeatDaemon(a.handleAgentWakeUp)
}

func (a *App) handleAgentWakeUp(agent db.Agent, runID uuid.UUID) {
	sessionID := "acp-session-" + agent.ID.String()
	swarmProvider := "gemini"
	if a.config != nil {
		active := a.config.GetActiveProviders()
		primary := strings.ToLower(strings.TrimSpace(a.config.PrimaryProvider))
		for _, p := range active {
			if p == primary {
				swarmProvider = p
				break
			}
		}
		if swarmProvider == "gemini" && len(active) > 0 && primary != "gemini" {
			swarmProvider = active[0]
		}
	}

	// 1. Buscar Ocupação Atual ou Nova Tarefa
	var issue db.Issue
	err := db.InstanceDB.Where("assignee_agent_id = ? AND status = ?", agent.ID, "in_progress").First(&issue).Error
	
	isNewTask := false
	if err != nil {
		// Tenta buscar algo novo na fila (TODO)
		err = db.InstanceDB.Where("status = ? AND (assignee_agent_id IS NULL OR assignee_agent_id = ?)", "todo", agent.ID).First(&issue).Error
		if err == nil {
			// Realiza Checkout Atômico
			lockedIssue, lockedErr := orchestration.CheckoutIssue(agent.ID, issue.ID)
			if lockedErr != nil {
				orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Conflito de checkout na fila.")
				return
			}
			issue = *lockedIssue
			isNewTask = true
		} else {
			// Nada para fazer
			orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Nenhuma tarefa pendente na fila.")
			return
		}
	}

	// 2. Iniciar ou Reutilizar Sessão ACP vinculada à Identidade
	err = a.executor.StartSession(a.ctx, swarmProvider, sessionID, "LATEST", agent.ID, &issue.ID, false, nil)
	if err != nil {
		orchestration.FinalizeHeartbeat(agent.ID, runID, false, "Erro ACP Swarm: "+err.Error())
		return
	}

	// 3. Construir Fat Context (Timeline + Metas)
	timeline, _ := orchestration.GetTimelineByIssue(issue.ID)
	historyStr := ""
	for i, c := range timeline {
		if i > 5 { break } // Apenas os últimos 5 para economia de tokens
		author := "Sistema"
		if c.AuthorAgentID != nil { author = "Agente" }
		historyStr += fmt.Sprintf("- %s: %s\n", author, c.Body)
	}

	prompt := ""
	if isNewTask {
		prompt = prompts.GetSwarmNewTaskPrompt(agent.Name, agent.Role, issue.Title, issue.Description)
	} else {
		prompt = prompts.GetSwarmContinuePrompt(agent.Name, agent.Role, issue.Title, historyStr)
	}

	// 3. Injetar Pulso de Inteligência no Agente Concorrente
	// O SendInput é assíncrono e lidará com o fluxo JSON-RPC
	err = a.executor.SendInput(sessionID, prompt, nil)
	if err != nil {
		orchestration.FinalizeHeartbeat(agent.ID, runID, false, "Erro ao injetar comando: "+err.Error())
		return
	}

	// Reporta que o pulso foi injetado com sucesso
	orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Agente acordado e processando tarefa.")
}

// Bindings para interação do Front-End (Wails)

// CreateAgent 'contrata' um novo agente no banco de dados corporativo local
func (a *App) CreateAgent(name, role string, listSkills string, budget int) string {
	agent := db.Agent{
		Name:               name,
		Role:               role,
		Capabilities:       listSkills,
		BudgetMonthlyCents: budget,
		Status:             "idle",
	}
	if err := db.InstanceDB.Create(&agent).Error; err != nil {
		return "Erro ao contratar: " + err.Error()
	}
	return "Agente " + name + " contratado e aguardando pulso de vida!"
}

// CreateTask adiciona uma nova tarefa atômica na fila da empresa
func (a *App) CreateTask(title, description, priority string) string {
	issue := db.Issue{
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      "todo",
	}
	if err := db.InstanceDB.Create(&issue).Error; err != nil {
		return "Erro ao criar tarefa: " + err.Error()
	}
	return "Tarefa '" + title + "' injetada na fila de orquestração."
}

// GetAgents retorna a lista de todos os agentes contratados
func (a *App) GetAgents() []db.Agent {
	var agents []db.Agent
	db.InstanceDB.Find(&agents)
	return agents
}

// GetIssues retorna todas as tarefas e seus respectivos donos (agentes)
func (a *App) GetIssues() []db.Issue {
	var issues []db.Issue
	// Preload carrega o objeto Agent associado via Foreign Key
	db.InstanceDB.Preload("AssigneeAgent").Find(&issues)
	return issues
}

// --- GOVERNANÇA V2 (METAS, TIMELINE E APROVAÇÕES) ---

// CreateGoal cria um novo objetivo estratégico para nortear o enxame
func (a *App) CreateGoal(title, description, level, parentIDStr, ownerIDStr string) string {
	var parentID *uuid.UUID
	if parentIDStr != "" {
		u, err := uuid.Parse(parentIDStr)
		if err == nil { parentID = &u }
	}
	var ownerID *uuid.UUID
	if ownerIDStr != "" {
		u, err := uuid.Parse(ownerIDStr)
		if err == nil { ownerID = &u }
	}

	goal := db.Goal{
		Title:        title,
		Description:  description,
		Level:        level,
		ParentID:     parentID,
		OwnerAgentID: ownerID,
	}
	if err := db.InstanceDB.Create(&goal).Error; err != nil {
		return "Erro ao criar meta: " + err.Error()
	}
	return "Meta '" + title + "' estabelecida no plano estratégico!"
}

// GetGoals lista a árvore de objetivos da empresa
func (a *App) GetGoals() []db.Goal {
	var goals []db.Goal
	db.InstanceDB.Find(&goals)
	return goals
}

// AddComment insere uma nota na linha do tempo de uma tarefa (Audit Chain)
func (a *App) AddComment(issueIDStr, body string) string {
	issueID, _ := uuid.Parse(issueIDStr)
	// Comentário manual via UI usa Actor System (uuid.Nil)
	err := orchestration.AddIssueComment(uuid.Nil, issueID, body)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Nota registrada na tarefa."
}

// GetIssueTimeline recupera a história completa de uma tarefa
func (a *App) GetIssueTimeline(issueIDStr string) []db.IssueComment {
	issueID, _ := uuid.Parse(issueIDStr)
	comments, _ := orchestration.GetTimelineByIssue(issueID)
	return comments
}

// ApproveAction libera um Portão de Aprovação (Board Decision)
func (a *App) ApproveAction(approvalIDStr, note string) string {
	id, _ := uuid.Parse(approvalIDStr)
	err := orchestration.ProcessApproval(id, true, note)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Ação aprovada e registrada na auditoria."
}

// RejectAction bloqueia permanentemente uma intenção da IA
func (a *App) RejectAction(approvalIDStr, note string) string {
	id, _ := uuid.Parse(approvalIDStr)
	err := orchestration.ProcessApproval(id, false, note)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Ação rejeitada. O agente permanecerá em pausa para reavaliação."
}

// --- SUITE EXECUTIVA (KPIs E DOCUMENTAÇÃO RAG) ---

// GetExecutiveSummary retorna os KPIs do Enxame para o Dashboard de Comando
func (a *App) GetExecutiveSummary() orchestration.ExecSummary {
	summary, _ := orchestration.GetExecutiveSummary()
	return summary
}

// GetDocuments retorna a lista de documentos (entregas) de uma tarefa
func (a *App) GetDocuments(issueIDStr string) []db.Document {
	issueID, _ := uuid.Parse(issueIDStr)
	var docs []db.Document
	db.InstanceDB.Where("issue_id = ?", issueID).Find(&docs)
	return docs
}

// UpsertDocument sincroniza um documento (PRD, Spec, Relatório) com o banco e o RAG
func (a *App) UpsertDocument(issueIDStr, title, body, change string) string {
	issueID, _ := uuid.Parse(issueIDStr)
	// Operação manual via UI usa Actor System (uuid.Nil)
	_, err := orchestration.UpsertDocument(uuid.Nil, issueID, title, body, change)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Documento '" + title + "' guardado e indexado para o RAG."
}

// --- GESTÃO DE SEGREDOS (AGENT VAULT) ---

// GetAgentSecrets retorna as chaves de API cadastradas para um agente
func (a *App) GetAgentSecrets(agentIDStr string) []db.AgentSecret {
	agentID, _ := uuid.Parse(agentIDStr)
	var secrets []db.AgentSecret
	db.InstanceDB.Where("agent_id = ?", agentID).Find(&secrets)
	return secrets
}

// UpdateAgentSecret salva ou atualiza uma credencial (ex: OPENAI_API_KEY) para um agente
func (a *App) UpdateAgentSecret(agentIDStr, key, value string) string {
	agentID, _ := uuid.Parse(agentIDStr)
	var secret db.AgentSecret
	err := db.InstanceDB.Where("agent_id = ? AND key = ?", agentID, key).First(&secret).Error
	
	if err != nil {
		secret = db.AgentSecret{AgentID: agentID, Key: key, Value: value}
		db.InstanceDB.Create(&secret)
	} else {
		secret.Value = value
		db.InstanceDB.Save(&secret)
	}
	return "Segredo '" + key + "' atualizado para o agente."
}

// GetPendingApprovals retorna todas as solicitações de aprovação que aguardam decisão humana.
func (a *App) GetPendingApprovals() []db.Approval {
	var approvals []db.Approval
	db.InstanceDB.Where("status = ?", "pending").Order("created_at DESC").Find(&approvals)
	return approvals
}
