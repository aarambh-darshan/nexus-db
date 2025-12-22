
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	export interface AppTypes {
		RouteId(): "/" | "/migrations" | "/query" | "/tables" | "/tables/[name]";
		RouteParams(): {
			"/tables/[name]": { name: string }
		};
		LayoutParams(): {
			"/": { name?: string };
			"/migrations": Record<string, never>;
			"/query": Record<string, never>;
			"/tables": { name?: string };
			"/tables/[name]": { name: string }
		};
		Pathname(): "/" | "/migrations" | "/migrations/" | "/query" | "/query/" | "/tables" | "/tables/" | `/tables/${string}` & {} | `/tables/${string}/` & {};
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): string & {};
	}
}