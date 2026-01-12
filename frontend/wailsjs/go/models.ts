export namespace main {
	
	export class AccountItem {
	    id: number;
	    phone: string;
	    name: string;
	    is_premium: boolean;
	    is_active: boolean;
	    create_at: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.phone = source["phone"];
	        this.name = source["name"];
	        this.is_premium = source["is_premium"];
	        this.is_active = source["is_active"];
	        this.create_at = source["create_at"];
	    }
	}

}

