export namespace ipc {
	
	export class OpenVPNStatus {
	    id: string;
	    owner_id?: string;
	    name: string;
	    status: string;
	    dev_type: string;
	    ip?: string;
	    rx_bytes: number;
	    tx_bytes: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenVPNStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.owner_id = source["owner_id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.dev_type = source["dev_type"];
	        this.ip = source["ip"];
	        this.rx_bytes = source["rx_bytes"];
	        this.tx_bytes = source["tx_bytes"];
	        this.error = source["error"];
	    }
	}
	export class PeerInfo {
	    public_key?: string;
	    endpoint: string;
	    allowed_ips: string[];
	    persistent_keepalive: number;
	
	    static createFrom(source: any = {}) {
	        return new PeerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.public_key = source["public_key"];
	        this.endpoint = source["endpoint"];
	        this.allowed_ips = source["allowed_ips"];
	        this.persistent_keepalive = source["persistent_keepalive"];
	    }
	}
	export class StatsInfo {
	    tunnel_id: string;
	    owner_id?: string;
	    rx_bytes: number;
	    tx_bytes: number;
	    last_handshake: number;
	
	    static createFrom(source: any = {}) {
	        return new StatsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tunnel_id = source["tunnel_id"];
	        this.owner_id = source["owner_id"];
	        this.rx_bytes = source["rx_bytes"];
	        this.tx_bytes = source["tx_bytes"];
	        this.last_handshake = source["last_handshake"];
	    }
	}
	export class TunnelInfo {
	    id: string;
	    owner_id?: string;
	    name: string;
	    status: string;
	    addresses: string[];
	    dns: string[];
	    mtu: number;
	    listen_port: number;
	    private_key?: string;
	    peers: PeerInfo[];
	    error?: string;
	    is_managed?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TunnelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.owner_id = source["owner_id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.addresses = source["addresses"];
	        this.dns = source["dns"];
	        this.mtu = source["mtu"];
	        this.listen_port = source["listen_port"];
	        this.private_key = source["private_key"];
	        this.peers = this.convertValues(source["peers"], PeerInfo);
	        this.error = source["error"];
	        this.is_managed = source["is_managed"];
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
	export class UpdateState {
	    state: string;
	    supported: boolean;
	    current_version: string;
	    latest_version?: string;
	    mandatory?: boolean;
	    notes?: string;
	    progress?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.supported = source["supported"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.mandatory = source["mandatory"];
	        this.notes = source["notes"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	    }
	}

}

export namespace main {
	
	export class ManagedLoginResult {
	    username: string;
	    is_admin: boolean;
	    require_totp: boolean;
	    totp_enabled: boolean;
	    push_auth_enabled: boolean;
	    push_request_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedLoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.is_admin = source["is_admin"];
	        this.require_totp = source["require_totp"];
	        this.totp_enabled = source["totp_enabled"];
	        this.push_auth_enabled = source["push_auth_enabled"];
	        this.push_request_id = source["push_request_id"];
	    }
	}
	export class ManagedSettings {
	    server_url: string;
	    username: string;
	    is_admin: boolean;
	    logged_in: boolean;
	    vpn_name: string;
	    totp_enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ManagedSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_url = source["server_url"];
	        this.username = source["username"];
	        this.is_admin = source["is_admin"];
	        this.logged_in = source["logged_in"];
	        this.vpn_name = source["vpn_name"];
	        this.totp_enabled = source["totp_enabled"];
	    }
	}
	export class OpenVPNProfileView {
	    id: string;
	    name: string;
	    source: string;
	    requires_totp: boolean;
	    auth_user_pass: boolean;
	    dev_type: string;
	    allow_custom_ip: boolean;
	    custom_ip?: string;
	    remembered_user?: string;
	    has_saved_password: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OpenVPNProfileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.requires_totp = source["requires_totp"];
	        this.auth_user_pass = source["auth_user_pass"];
	        this.dev_type = source["dev_type"];
	        this.allow_custom_ip = source["allow_custom_ip"];
	        this.custom_ip = source["custom_ip"];
	        this.remembered_user = source["remembered_user"];
	        this.has_saved_password = source["has_saved_password"];
	    }
	}

}

export namespace managed {
	
	export class ServerInfo {
	    id: string;
	    name: string;
	    endpoint: string;
	    port: number;
	    public_key: string;
	    subnet: string;
	    dns?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.endpoint = source["endpoint"];
	        this.port = source["port"];
	        this.public_key = source["public_key"];
	        this.subnet = source["subnet"];
	        this.dns = source["dns"];
	    }
	}

}

