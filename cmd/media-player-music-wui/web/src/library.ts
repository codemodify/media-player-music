// The library: its artists, albums, genres and years, down to their
// tracks (Browse), and what a search finds (Search); each played now or
// added to the queue (Add, with the tracks' paths).

import type { App } from "./app.ts";
import { $, button, el, plural, time, toast } from "./dom.ts";
import type { Browse, Group, Library, Track } from "./musicapi.ts";
import { fileName } from "./player.ts";

type By = "artists" | "albums" | "genres" | "years";

// A Place is a list the library shows: Browse's question, and its name in
// the crumbs.
interface Place {
	label: string;
	browse: Browse;
}

const page = 200; // a list's rows asked at once

export class LibraryView {
	private readonly body = $("library-body");
	private readonly search = $<HTMLInputElement>("search");
	private places: Place[] = [];
	private shown = false; // the view is shown: what it lists is asked
	private stale = true; // the library changed since the list was asked
	private version = -1;
	private asking = 0; // the newest question's number: older answers are dropped
	private searchTimer = 0;
	private reloadTimer = 0;
	private focusList = false; // a group was opened: the focus goes to its list

	constructor(private readonly app: App) {
		for (const choice of $("browse-by").querySelectorAll<HTMLButtonElement>("button")) {
			choice.addEventListener("click", () => {
				this.search.value = "";
				this.root(choice.dataset.by as By);
			});
		}
		this.search.addEventListener("input", () => {
			clearTimeout(this.searchTimer);
			this.searchTimer = window.setTimeout(() => this.load(), 250);
		});
		this.search.addEventListener("keydown", (event) => {
			if (event.key === "Escape" && this.search.value) {
				event.preventDefault();
				event.stopPropagation();
				this.search.value = "";
				this.load();
			} else if (event.key === "Enter") {
				clearTimeout(this.searchTimer);
				this.load();
			}
		});
		const first = $<HTMLInputElement>("first-folder");
		const addFirst = async () => {
			const path = first.value.trim();
			if (!path) {
				first.focus();
				return;
			}
			if ((await app.call((p) => p.AddFolder(path))) !== undefined) {
				first.value = "";
				toast(`Reading ${path}…`);
			}
		};
		$("add-first-folder").addEventListener("click", () => void addFirst());
		first.addEventListener("keydown", (event) => {
			if (event.key === "Enter") void addFirst();
		});
		this.root("artists");
	}

	// visible says whether the view is shown: it asks for its list when it
	// is, and the library changed.
	visible(shown: boolean): void {
		this.shown = shown;
		if (shown && this.stale) this.load();
	}

	// focusSearch puts the focus in the search.
	focusSearch(): void {
		this.search.focus();
		this.search.select();
	}

	// render shows the library in short: the empty state without folders;
	// the list asked again when it changed (at most every two seconds while
	// it is read).
	render(library: Library): void {
		const none = (library.folders ?? []).length === 0;
		$("library-empty").hidden = !none;
		this.body.hidden = none;
		$("crumbs").hidden = none;
		if (library.version === this.version) return;
		this.version = library.version;
		this.stale = true;
		if (!this.shown || none) return;
		if (this.reloadTimer) return;
		this.reloadTimer = window.setTimeout(() => {
			this.reloadTimer = 0;
			if (this.stale && this.shown) this.load();
		}, library.scanning ? 2000 : 0);
	}

	private root(by: By): void {
		for (const choice of $("browse-by").querySelectorAll<HTMLButtonElement>("button")) {
			choice.setAttribute("aria-pressed", String(choice.dataset.by === by));
		}
		this.places = [{ label: { artists: "Artists", albums: "Albums", genres: "Genres", years: "Years" }[by], browse: { by } }];
		this.load();
	}

	private go(place: Place): void {
		this.search.value = "";
		this.places.push(place);
		this.focusList = true;
		this.load();
	}

	// load asks for what is shown: the search's findings, or the place's list.
	private load(): void {
		this.stale = false;
		const ask = ++this.asking;
		const text = this.search.value.trim();
		if (text) {
			this.renderCrumbs(true);
			void this.app.ask((p) => p.Search({ text })).then((found) => {
				if (ask !== this.asking || !found) return;
				const parts: Node[] = [];
				if (found.artists?.length) {
					parts.push(el("h3", { className: "section" }, "Artists"), this.groups(found.artists, "artists", { by: "artists" }));
				}
				if (found.albums?.length) {
					parts.push(el("h3", { className: "section" }, "Albums"), this.groups(found.albums, "albums", { by: "albums" }));
				}
				if (found.tracks?.length) {
					parts.push(el("h3", { className: "section" }, "Tracks"), this.trackBar(found.tracks), this.trackList(found.tracks));
				}
				if (parts.length === 0) parts.push(el("p", { className: "empty" }, `Nothing found for “${text}”.`));
				if (found.more) parts.push(el("p", { className: "note more" }, "There is more: say more of what you look for."));
				this.body.replaceChildren(...parts);
			});
			return;
		}
		this.renderCrumbs(false);
		const place = this.places[this.places.length - 1];
		void this.app.ask((p) => p.Browse({ ...place.browse, limit: page })).then((browsed) => {
			if (ask !== this.asking || !browsed) return;
			this.body.replaceChildren();
			this.show(place, browsed.groups ?? [], browsed.tracks ?? [], browsed.total);
		});
	}

	// show adds a page of the place's list to what is shown, and a button
	// for the next page while there is one.
	private show(place: Place, groups: Group[], tracks: Track[], total: number): void {
		const focus = this.focusList;
		this.focusList = false;
		this.body.querySelector(".more")?.remove();
		const isTracks = place.browse.by === "tracks";
		let list = this.body.querySelector<HTMLOListElement>("ol.list");
		const shown = (list?.children.length ?? 0) + (isTracks ? tracks.length : groups.length);
		if (!list) {
			if (shown === 0) {
				this.body.append(el("p", { className: "empty" }, "Nothing here yet."));
				return;
			}
			const filter = place.browse;
			const parts: Node[] = [el("p", { className: "note" }, `${plural(total, isTracks ? "track" : noun(filter.by))}`)];
			if (isTracks || this.places.length > 1) parts.push(this.filterBar(filter));
			list = isTracks ? this.trackList(tracks) : this.groups(groups, filter.by as By, filter);
			this.body.append(...parts, list);
			if (focus) list.querySelector<HTMLButtonElement>("button.name:not(:disabled)")?.focus();
		} else {
			const more = isTracks ? this.trackList(tracks) : this.groups(groups, place.browse.by as By, place.browse);
			list.append(...more.children);
		}
		if (shown < total) {
			const next = el("button", { type: "button", className: "btn" }, `Show more (${(total - shown).toLocaleString()} left)`);
			next.addEventListener("click", () => {
				const ask = this.asking;
				void this.app.ask((p) => p.Browse({ ...place.browse, offset: shown, limit: page })).then((browsed) => {
					if (ask !== this.asking || !browsed) return;
					this.show(place, browsed.groups ?? [], browsed.tracks ?? [], browsed.total);
				});
			});
			this.body.append(el("div", { className: "more" }, next));
		}
	}

	private renderCrumbs(searching: boolean): void {
		const crumbs = $("crumbs");
		if (searching) {
			crumbs.replaceChildren(el("span", { "aria-current": "page" }, "Search"));
			return;
		}
		const parts: Node[] = [];
		this.places.forEach((place, index) => {
			if (index > 0) parts.push(el("span", { className: "sep", "aria-hidden": "true" }, "›"));
			if (index === this.places.length - 1) {
				parts.push(el("span", { "aria-current": "page" }, place.label));
				return;
			}
			const back = el("button", { type: "button" }, place.label);
			back.addEventListener("click", () => {
				this.places = this.places.slice(0, index + 1);
				this.load();
			});
			parts.push(back);
		});
		crumbs.replaceChildren(...parts);
	}

	// groups lists artists, albums, genres or years, each opened (its next
	// list), played or added; within the tracks of filter.
	private groups(groups: Group[], by: By, filter: Browse): HTMLOListElement {
		return el(
			"ol",
			{ className: "list" },
			...groups.map((group) => {
				const name = groupName(group, by);
				const next = this.inside(group, by, filter);
				const open = el("button", { type: "button", className: "name" }, name);
				open.addEventListener("click", () => this.go({ label: name, browse: next }));
				const meta = [by === "albums" ? group.artist : "", by === "albums" && group.year ? String(group.year) : "", plural(group.tracks, "track")]
					.filter(Boolean)
					.join(" · ");
				const tracks = { ...next, by: "tracks" };
				const play = button("▶", `Play ${name}`, () => void this.add(tracks, true));
				const add = button("＋", `Add ${name} to the queue`, () => void this.add(tracks, false));
				return el(
					"li",
					{},
					el("span", { className: "num" }, ""),
					el("div", { className: "what" }, open, el("div", { className: "meta" }, meta)),
					el("span", { className: "len" }, time(group.lengthMs)),
					el("span", { className: "acts" }, play, add),
				);
			}),
		);
	}

	// inside is the question for what is in a group: an artist's albums, an
	// album's tracks, a genre's artists, a year's albums. A group whose
	// tags do not say (no name, no year) is asked for by Browse's unknown:
	// that field empty.
	private inside(group: Group, by: By, filter: Browse): Browse {
		const unknown = [...(filter.unknown ?? [])];
		const base: Browse = { by: "tracks", artist: filter.artist, album: filter.album, genre: filter.genre, year: filter.year };
		const field = { artists: "artist", albums: "album", genres: "genre", years: "year" }[by];
		const named = by === "years" ? (group.year ?? 0) > 0 : group.name !== "";
		if (!named && !unknown.includes(field)) unknown.push(field);
		let next: Browse;
		switch (by) {
			case "artists":
				next = { ...base, by: "albums", artist: group.name || undefined };
				break;
			case "albums":
				next = { ...base, by: "tracks", album: group.name || undefined, artist: group.artist || filter.artist };
				break;
			case "genres":
				next = { ...base, by: "artists", genre: group.name || undefined };
				break;
			case "years":
				next = { ...base, by: "albums", year: group.year || undefined };
				break;
		}
		if (unknown.length) next.unknown = unknown;
		return next;
	}

	// filterBar plays or adds every track of the place shown.
	private filterBar(filter: Browse): HTMLElement {
		const tracks = { ...filter, by: "tracks", offset: undefined, limit: undefined };
		const play = el("button", { type: "button", className: "btn primary" }, "Play all");
		play.addEventListener("click", () => void this.add(tracks, true));
		const add = el("button", { type: "button", className: "btn" }, "Add all to the queue");
		add.addEventListener("click", () => void this.add(tracks, false));
		return el("div", { className: "bar" }, play, add);
	}

	// trackBar plays or adds tracks found.
	private trackBar(tracks: Track[]): HTMLElement {
		const paths = tracks.map((track) => track.path);
		const play = el("button", { type: "button", className: "btn primary" }, "Play these");
		play.addEventListener("click", () => void this.addPaths(paths, true));
		const add = el("button", { type: "button", className: "btn" }, "Add these to the queue");
		add.addEventListener("click", () => void this.addPaths(paths, false));
		return el("div", { className: "bar" }, play, add);
	}

	private trackList(tracks: Track[]): HTMLOListElement {
		return el(
			"ol",
			{ className: "list" },
			...tracks.map((track) => {
				const title = track.title || fileName(track.path);
				const open = el("button", { type: "button", className: "name", title: `Play ${title} now` }, title);
				open.addEventListener("click", () => void this.playNow(track));
				const meta = [track.artist, track.album, track.year ? String(track.year) : ""].filter(Boolean).join(" · ");
				const item = el(
					"li",
					{},
					el("span", { className: "num" }, track.number ? String(track.number) : ""),
					el("div", { className: "what" }, open, meta ? el("div", { className: "meta" }, meta) : null),
					el("span", { className: "len" }, track.lengthMs ? time(track.lengthMs) : ""),
					el(
						"span",
						{ className: "acts" },
						button("▶", `Play ${title} now`, () => void this.playNow(track)),
						button("＋", `Add ${title} to the queue`, () => void this.addPaths([track.path], false)),
					),
				);
				item.title = track.path;
				return item;
			}),
		);
	}

	// add plays (in the queue's place) or adds the tracks Browse finds.
	private async add(tracks: Browse, replace: boolean): Promise<void> {
		const browsed = await this.app.ask((p) => p.Browse(tracks));
		if (!browsed) return;
		await this.addPaths((browsed.tracks ?? []).map((track) => track.path), replace);
	}

	private async addPaths(paths: string[], replace: boolean): Promise<void> {
		if (paths.length === 0) return;
		const added = await this.app.call((p) => p.Add({ paths, replace }));
		if (!added) return;
		toast(
			(replace ? `Playing ${plural(added.tracks, "track")}` : `Added ${plural(added.tracks, "track")} to the queue`) +
				(added.skipped ? `; ${added.skipped}` : ""),
		);
	}

	// playNow adds a track at the queue's end and plays it: the queue stays.
	private async playNow(track: Track): Promise<void> {
		const added = await this.app.call((p) => p.Add({ paths: [track.path] }));
		if (!added || added.tracks === 0) return;
		const index = added.state.queue.length - added.tracks;
		await this.app.call((p) => p.Play(index));
	}
}

function noun(by: string): string {
	return { artists: "artist", albums: "album", genres: "genre", years: "year" }[by] ?? "item";
}

function groupName(group: Group, by: By): string {
	if (by === "years") return group.year ? String(group.year) : "Unknown year";
	if (group.name) return group.name;
	return { artists: "Unknown artist", albums: "Unknown album", genres: "Unknown genre" }[by];
}
