export namespace config {
	
	export class Config {
	    obsidian_vault_path: string;
	    qdrant_url: string;
	    gemini_api_key: string;
	    use_gemini_api_key: boolean;
	    claude_api_key: string;
	    use_claude_api_key: boolean;
	    active_agent: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.obsidian_vault_path = source["obsidian_vault_path"];
	        this.qdrant_url = source["qdrant_url"];
	        this.gemini_api_key = source["gemini_api_key"];
	        this.use_gemini_api_key = source["use_gemini_api_key"];
	        this.claude_api_key = source["claude_api_key"];
	        this.use_claude_api_key = source["use_claude_api_key"];
	        this.active_agent = source["active_agent"];
	    }
	}

}

