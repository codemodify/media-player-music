// What the page's views share: the calls to the daemon, and the views
// shown.

import type { Library, Player, State } from "./musicapi.ts";

export type View = "queue" | "library" | "settings";

export interface App {
	// call calls the daemon after the calls before it, in the order the
	// person did things; a State, an Added's state or a Library it answers
	// is shown at once; an error is said, and the answer is undefined.
	call<T>(work: (player: Player) => Promise<T>): Promise<T | undefined>;
	// ask calls the daemon at once, beside the calls in order: for what
	// only reads (Browse, Search); an error is said, the answer undefined.
	ask<T>(work: (player: Player) => Promise<T>): Promise<T | undefined>;
	// state and library are the newest shown.
	state(): State | undefined;
	library(): Library | undefined;
	// show shows a view.
	show(view: View): void;
}
