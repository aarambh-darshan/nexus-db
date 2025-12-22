export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.UsKeJUJ8.js",app:"_app/immutable/entry/app.BNIVksfc.js",imports:["_app/immutable/entry/start.UsKeJUJ8.js","_app/immutable/chunks/wzx0znVI.js","_app/immutable/chunks/-vKrXhex.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/CHYVOm2z.js","_app/immutable/entry/app.BNIVksfc.js","_app/immutable/chunks/-vKrXhex.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/FTzCPRq0.js","_app/immutable/chunks/Bzak7iHL.js","_app/immutable/chunks/CHYVOm2z.js","_app/immutable/chunks/D1rrPloc.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js')),
			__memo(() => import('./nodes/3.js')),
			__memo(() => import('./nodes/4.js')),
			__memo(() => import('./nodes/5.js')),
			__memo(() => import('./nodes/6.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			},
			{
				id: "/migrations",
				pattern: /^\/migrations\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 3 },
				endpoint: null
			},
			{
				id: "/query",
				pattern: /^\/query\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 4 },
				endpoint: null
			},
			{
				id: "/tables",
				pattern: /^\/tables\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 5 },
				endpoint: null
			},
			{
				id: "/tables/[name]",
				pattern: /^\/tables\/([^/]+?)\/?$/,
				params: [{"name":"name","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 6 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
