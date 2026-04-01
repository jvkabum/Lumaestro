export namespace agents {
	
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
	    gemini_accounts: GeminiAccount[];
	    claude_api_key: string;
	    use_claude_api_key: boolean;
	    active_agent: string;
	    auto_start_agents: string[];
	    agent_language: string;
	    graph_depth: number;
	    graph_neighbor_limit: number;
	    graph_context_limit: number;
	    security: SecurityConfig;
	
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
	        this.gemini_accounts = this.convertValues(source["gemini_accounts"], GeminiAccount);
	        this.claude_api_key = source["claude_api_key"];
	        this.use_claude_api_key = source["use_claude_api_key"];
	        this.active_agent = source["active_agent"];
	        this.auto_start_agents = source["auto_start_agents"];
	        this.agent_language = source["agent_language"];
	        this.graph_depth = source["graph_depth"];
	        this.graph_neighbor_limit = source["graph_neighbor_limit"];
	        this.graph_context_limit = source["graph_context_limit"];
	        this.security = this.convertValues(source["security"], SecurityConfig);
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

