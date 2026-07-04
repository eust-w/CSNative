export namespace app {

	export class ConfigResponse {
	    provider: string;
	    proxy_port: number;
	    sandbox_port: number;
	    public_port: number;
	    public_base_url: string;
	    mode: string;
	    keys: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new ConfigResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.proxy_port = source["proxy_port"];
	        this.sandbox_port = source["sandbox_port"];
	        this.public_port = source["public_port"];
	        this.public_base_url = source["public_base_url"];
	        this.mode = source["mode"];
	        this.keys = source["keys"];
	    }
	}
	export class ProviderView {
	    id?: string;
	    display_name?: string;
	    adapter?: string;
	    base_url?: string;
	    api_key?: string;
	    key?: string;
	    default_model?: string;
	    models?: config.ProviderModel[];
	    model_map?: Record<string, string>;
	    max_tokens_cap?: Record<string, number>;
	    enabled: boolean;
	    disabled?: boolean;
	    builtin: boolean;
	    verified?: boolean;
	    last_error?: string;
	    has_key: boolean;
	    key_masked: string;
	    active: boolean;

	    static createFrom(source: any = {}) {
	        return new ProviderView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.adapter = source["adapter"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.key = source["key"];
	        this.default_model = source["default_model"];
	        this.models = this.convertValues(source["models"], config.ProviderModel);
	        this.model_map = source["model_map"];
	        this.max_tokens_cap = source["max_tokens_cap"];
	        this.enabled = source["enabled"];
	        this.disabled = source["disabled"];
	        this.builtin = source["builtin"];
	        this.verified = source["verified"];
	        this.last_error = source["last_error"];
	        this.has_key = source["has_key"];
	        this.key_masked = source["key_masked"];
	        this.active = source["active"];
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
	export class StatusResponse {
	    proxy: string;
	    sandbox: string;
	    upstream: string;
	    public: string;

	    static createFrom(source: any = {}) {
	        return new StatusResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxy = source["proxy"];
	        this.sandbox = source["sandbox"];
	        this.upstream = source["upstream"];
	        this.public = source["public"];
	    }
	}
	export class UISettings {
	    provider: string;
	    proxy_port: number;
	    sandbox_port: number;
	    public_port: number;
	    public_base_url: string;

	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.proxy_port = source["proxy_port"];
	        this.sandbox_port = source["sandbox_port"];
	        this.public_port = source["public_port"];
	        this.public_base_url = source["public_base_url"];
	    }
	}

}

export namespace config {

	export class ProviderModel {
	    id: string;
	    display_name: string;

	    static createFrom(source: any = {}) {
	        return new ProviderModel(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	    }
	}
	export class ProviderConfig {
	    id?: string;
	    display_name?: string;
	    adapter?: string;
	    base_url?: string;
	    api_key?: string;
	    key?: string;
	    default_model?: string;
	    models?: ProviderModel[];
	    model_map?: Record<string, string>;
	    max_tokens_cap?: Record<string, number>;
	    enabled: boolean;
	    disabled?: boolean;
	    builtin: boolean;
	    verified?: boolean;
	    last_error?: string;

	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.adapter = source["adapter"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.key = source["key"];
	        this.default_model = source["default_model"];
	        this.models = this.convertValues(source["models"], ProviderModel);
	        this.model_map = source["model_map"];
	        this.max_tokens_cap = source["max_tokens_cap"];
	        this.enabled = source["enabled"];
	        this.disabled = source["disabled"];
	        this.builtin = source["builtin"];
	        this.verified = source["verified"];
	        this.last_error = source["last_error"];
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
