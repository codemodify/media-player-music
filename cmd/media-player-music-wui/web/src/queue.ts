// The queue: its tracks, the one playing marked; play one, move it,
// remove it; clear the queue, save it as a playlist.

import type { App } from "./app.ts";
import { $, button, el, plural, time, toast } from "./dom.ts";
import { fileName } from "./player.ts";
import type { Queue, State, Track } from "./musicapi.ts";

export class QueueView {
	private readonly list = $<HTMLOListElement>("queue");
	private version = -1;
	private tracks: Track[] = [];
	private current = -1;
	private playing = { index: -1, version: -1 }; // the newest state's track and queue
	private clearAsked = 0;
	// the row and its action to focus once the queue is drawn again
	private focusAfter: { index: number; action: string } | undefined;

	constructor(private readonly app: App) {
		const path = $<HTMLInputElement>("playlist-path");
		const save = async () => {
			const where = path.value.trim();
			if (!where) {
				toast("Type where to save the playlist, on the player's machine (….m3u8 or ….pls)");
				path.focus();
				return;
			}
			if ((await app.call((p) => p.SavePlaylist(where))) !== undefined) toast(`Saved the queue as ${where}`);
		};
		$("save-playlist").addEventListener("click", () => void save());
		path.addEventListener("keydown", (event) => {
			if (event.key === "Enter") void save();
		});
		const clear = $<HTMLButtonElement>("clear");
		clear.addEventListener("click", () => {
			// asked twice: a queue is long to make again
			if (Date.now() - this.clearAsked > 4000) {
				this.clearAsked = Date.now();
				clear.textContent = "Clear the queue?";
				setTimeout(() => (clear.textContent = "Clear"), 4000);
				return;
			}
			this.clearAsked = 0;
			clear.textContent = "Clear";
			void app.call((p) => p.Clear());
		});
		$("go-library").addEventListener("click", () => app.show("library"));
	}

	// reset has the next queue drawn, whatever its version.
	reset(): void {
		this.version = -1;
	}

	// render shows the queue, drawn again when it changed.
	render(queue: Queue): void {
		if (queue.version === this.version) return;
		this.version = queue.version;
		this.tracks = queue.tracks ?? [];
		const rows = this.tracks.map((track, index) => this.row(track, index));
		this.list.replaceChildren(...rows);
		const empty = this.tracks.length === 0;
		$("queue-empty").hidden = !empty;
		this.list.hidden = empty;
		$<HTMLButtonElement>("clear").disabled = empty;
		$<HTMLButtonElement>("save-playlist").disabled = empty;
		const total = this.tracks.reduce((sum, track) => sum + (track.lengthMs ?? 0), 0);
		$("queue-summary").textContent = empty ? "" : `${plural(this.tracks.length, "track")}, ${time(total)}`;
		$("queue-count").textContent = empty ? "" : String(this.tracks.length);
		this.mark(this.playing.version === this.version ? this.playing.index : -1, true);
		if (this.focusAfter) {
			const { index, action } = this.focusAfter;
			this.focusAfter = undefined;
			const target = rows[index]?.querySelector<HTMLButtonElement>(`[data-action=${action}]:not(:disabled)`) ?? rows[index]?.querySelector<HTMLButtonElement>("button.name");
			target?.focus();
		}
	}

	// state marks the track playing.
	state(state: State): void {
		this.playing = { index: state.index, version: state.queue.version };
		if (state.queue.version !== this.version) return; // the queue it is in comes
		this.mark(state.index, false);
	}

	private mark(index: number, always: boolean): void {
		if (index === this.current && !always) return;
		this.list.children[this.current]?.classList.remove("current");
		this.list.children[this.current]?.removeAttribute("aria-current");
		this.current = index;
		const row = this.list.children[index];
		if (row) {
			row.classList.add("current");
			row.setAttribute("aria-current", "true");
		}
	}

	private row(track: Track, index: number): HTMLLIElement {
		const last = this.tracks.length - 1;
		const title = track.title || fileName(track.path);
		const play = el("button", { type: "button", className: "name", title: `Play ${title}` }, title);
		play.addEventListener("click", () => void this.app.call((p) => p.Play(index)));
		const meta = [track.artist, track.album].filter(Boolean).join(" — ");
		const move = (to: number, action: string) => () => {
			this.focusAfter = { index: to, action };
			void this.app.call((p) => p.Move({ from: index, to }));
		};
		const up = button("▲", `Move ${title} up`, move(index - 1, "up"));
		up.dataset.action = "up";
		up.disabled = index === 0;
		const down = button("▼", `Move ${title} down`, move(index + 1, "down"));
		down.dataset.action = "down";
		down.disabled = index === last;
		const remove = button("✕", `Remove ${title} from the queue`, () => {
			this.focusAfter = { index: Math.min(index, last - 1), action: "remove" };
			void this.app.call((p) => p.Remove([index]));
		});
		remove.dataset.action = "remove";
		const item = el(
			"li",
			{},
			el("span", { className: "num" }, String(index + 1)),
			el("div", { className: "what" }, play, meta ? el("div", { className: "meta" }, meta) : null),
			el("span", { className: "len" }, track.lengthMs ? time(track.lengthMs) : ""),
			el("span", { className: "acts" }, up, down, remove),
		);
		item.title = track.path;
		// Delete removes the row's track, Alt+↑/↓ moves it
		item.addEventListener("keydown", (event) => {
			if (event.key === "Delete") {
				event.preventDefault();
				remove.click();
			} else if (event.altKey && event.key === "ArrowUp" && index > 0) {
				event.preventDefault();
				event.stopPropagation();
				move(index - 1, "up")();
			} else if (event.altKey && event.key === "ArrowDown" && index < last) {
				event.preventDefault();
				event.stopPropagation();
				move(index + 1, "down")();
			}
		});
		return item;
	}
}
