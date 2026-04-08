export namespace acp {
	
	export class SessionInfo {
	    sessionId: string;
	    title: string;
	    createdAt: string;
	    updatedAt: string;
	    file: string;
	    isCurrentSession: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.title = source["title"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.file = source["file"];
	        this.isCurrentSession = source["isCurrentSession"];
	    }
	}

}

export namespace config {
	
	export class SecurityConfig {
	    allow_read: boolean;
	    allow_write: boolean;
	    allow_create: boolean;
	    allow_delete: boolean;
	    allow_move: boolean;
	    allow_run_commands: boolean;
	    full_machine_access: boolean;
	    workspaces: string[];
	
	    static createFrom(source: any = {}) {
	        return new SecurityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allow_read = source["allow_read"];
	        this.allow_write = source["allow_write"];
	        this.allow_create = source["allow_create"];
	        this.allow_delete = source["allow_delete"];
	        this.allow_move = source["allow_move"];
	        this.allow_run_commands = source["allow_run_commands"];
	        this.full_machine_access = source["full_machine_access"];
	        this.workspaces = source["workspaces"];
	    }
	}
	export class ProjectScan {
	    path: string;
	    core_node: string;
	    include_code: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProjectScan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.core_node = source["core_node"];
	        this.include_code = source["include_code"];
	    }
	}
	export class GeminiAccount {
	    name: string;
	    home_dir: string;
	    active: boolean;
	    exhausted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GeminiAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.home_dir = source["home_dir"];
	        this.active = source["active"];
	        this.exhausted = source["exhausted"];
	    }
	}
	export class Config {
	    obsidian_vault_path: string;
	    qdrant_url: string;
	    qdrant_api_key: string;
	    gemini_api_key: string;
	    use_gemini_api_key: boolean;
	    gemini_key_index: number;
	    gemini_accounts: GeminiAccount[];
	    claude_api_key: string;
	    use_claude_api_key: boolean;
	    active_agent: string;
	    auto_start_agents: string[];
	    agent_language: string;
	    max_concurrent_agents: number;
	    external_projects: ProjectScan[];
	    graph_depth: number;
	    graph_neighbor_limit: number;
	    graph_context_limit: number;
	    security: SecurityConfig;
	    lightning_enabled: boolean;
	    lightning_proxy_port: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.obsidian_vault_path = source["obsidian_vault_path"];
	        this.qdrant_url = source["qdrant_url"];
	        this.qdrant_api_key = source["qdrant_api_key"];
	        this.gemini_api_key = source["gemini_api_key"];
	        this.use_gemini_api_key = source["use_gemini_api_key"];
	        this.gemini_key_index = source["gemini_key_index"];
	        this.gemini_accounts = this.convertValues(source["gemini_accounts"], GeminiAccount);
	        this.claude_api_key = source["claude_api_key"];
	        this.use_claude_api_key = source["use_claude_api_key"];
	        this.active_agent = source["active_agent"];
	        this.auto_start_agents = source["auto_start_agents"];
	        this.agent_language = source["agent_language"];
	        this.max_concurrent_agents = source["max_concurrent_agents"];
	        this.external_projects = this.convertValues(source["external_projects"], ProjectScan);
	        this.graph_depth = source["graph_depth"];
	        this.graph_neighbor_limit = source["graph_neighbor_limit"];
	        this.graph_context_limit = source["graph_context_limit"];
	        this.security = this.convertValues(source["security"], SecurityConfig);
	        this.lightning_enabled = source["lightning_enabled"];
	        this.lightning_proxy_port = source["lightning_proxy_port"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace db {
	
	export class Timestamp {
	
	
	    static createFrom(source: any = {}) {
	        return new Timestamp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Agent {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    name: string;
	    role: string;
	    status: string;
	    reports_to?: number[];
	    capabilities: string;
	    budget_monthly_cents: number;
	    spent_monthly_cents: number;
	    last_heartbeat_at: Timestamp;
	
	    static createFrom(source: any = {}) {
	        return new Agent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.name = source["name"];
	        this.role = source["role"];
	        this.status = source["status"];
	        this.reports_to = source["reports_to"];
	        this.capabilities = source["capabilities"];
	        this.budget_monthly_cents = source["budget_monthly_cents"];
	        this.spent_monthly_cents = source["spent_monthly_cents"];
	        this.last_heartbeat_at = this.convertValues(source["last_heartbeat_at"], Timestamp);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AgentSecret {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    agent_id: number[];
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentSecret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.agent_id = source["agent_id"];
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Approval {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    type: string;
	    requested_by_agent_id?: number[];
	    status: string;
	    payload: string;
	    decision_note: string;
	    decided_at?: Timestamp;
	
	    static createFrom(source: any = {}) {
	        return new Approval(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.type = source["type"];
	        this.requested_by_agent_id = source["requested_by_agent_id"];
	        this.status = source["status"];
	        this.payload = source["payload"];
	        this.decision_note = source["decision_note"];
	        this.decided_at = this.convertValues(source["decided_at"], Timestamp);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Document {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    title: string;
	    format: string;
	    latest_body: string;
	    latest_revision_number: number;
	    issue_id?: number[];
	    created_by_agent_id?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Document(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.title = source["title"];
	        this.format = source["format"];
	        this.latest_body = source["latest_body"];
	        this.latest_revision_number = source["latest_revision_number"];
	        this.issue_id = source["issue_id"];
	        this.created_by_agent_id = source["created_by_agent_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Goal {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    title: string;
	    description: string;
	    level: string;
	    parent_id?: number[];
	    owner_agent_id?: number[];
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Goal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.title = source["title"];
	        this.description = source["description"];
	        this.level = source["level"];
	        this.parent_id = source["parent_id"];
	        this.owner_agent_id = source["owner_agent_id"];
	        this.status = source["status"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Issue {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    project_id?: number[];
	    goal_id?: number[];
	    parent_id?: number[];
	    title: string;
	    description: string;
	    status: string;
	    priority: string;
	    assignee_agent_id?: number[];
	    assignee_agent?: Agent;
	    created_by_agent_id?: number[];
	    started_at?: Timestamp;
	    completed_at?: Timestamp;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.project_id = source["project_id"];
	        this.goal_id = source["goal_id"];
	        this.parent_id = source["parent_id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.assignee_agent_id = source["assignee_agent_id"];
	        this.assignee_agent = this.convertValues(source["assignee_agent"], Agent);
	        this.created_by_agent_id = source["created_by_agent_id"];
	        this.started_at = this.convertValues(source["started_at"], Timestamp);
	        this.completed_at = this.convertValues(source["completed_at"], Timestamp);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IssueComment {
	    id: number[];
	    created_at: Timestamp;
	    updated_at: Timestamp;
	    issue_id: number[];
	    author_agent_id?: number[];
	    author_agent?: Agent;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new IssueComment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], Timestamp);
	        this.updated_at = this.convertValues(source["updated_at"], Timestamp);
	        this.issue_id = source["issue_id"];
	        this.author_agent_id = source["author_agent_id"];
	        this.author_agent = this.convertValues(source["author_agent"], Agent);
	        this.body = source["body"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace lightning {
	
	export class DuckDBStore {
	
	
	    static createFrom(source: any = {}) {
	        return new DuckDBStore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace orchestration {
	
	export class ExecSummary {
	    total_spent_cents: number;
	    active_agents: number;
	    paused_agents: number;
	    open_issues: number;
	    done_issues: number;
	    pending_approvals: number;
	
	    static createFrom(source: any = {}) {
	        return new ExecSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_spent_cents = source["total_spent_cents"];
	        this.active_agents = source["active_agents"];
	        this.paused_agents = source["paused_agents"];
	        this.open_issues = source["open_issues"];
	        this.done_issues = source["done_issues"];
	        this.pending_approvals = source["pending_approvals"];
	    }
	}

}

