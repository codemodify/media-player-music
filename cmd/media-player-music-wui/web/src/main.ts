// media-player-music's web UI: a remote control of the player
// media-player-musicd owns, the same one the desktop and terminal UIs show.
// The page connects to the daemon through the server that serves it
// (/clients), shows the states the daemon keeps (the player, the queue,
// where the track is, the library in short), and passes on what is done.
// The music is heard where the daemon runs.

import { connect, NotConnectedError, type Status } from "app-client-typescript";
import type { App, View } from "./app.ts";
import { $, isField, toast } from "./dom.ts";
import { LibraryView } from "./library.ts";
import {
	CommandMute,
	CommandNext,
	CommandPlayPause,
	CommandPrevious,
	CommandRepeat,
	CommandSeekBack,
	CommandSeekForward,
	CommandShuffle,
	CommandStop,
	CommandVolumeDown,
	CommandVolumeUp,
	LibraryEvent,
	newPlayer,
	NowEvent,
	playerStates,
	QueueEvent,
	StateEvent,
} from "./musicapi.ts";
import type { Library, Player, State } from "./musicapi.ts";
import { PlayerView } from "./player.ts";
import { QueueView } from "./queue.ts";
import { SettingsView } from "./settings.ts";

let player: Player | undefined;
let state: State | undefined; // the newest state shown
let library: Library | undefined;
// how the connection stands, "" once connected, and whether that is a problem
let link = { text: "connecting…", problem: false };
let dismissed = ""; // the error the person put away

// --- the calls ------------------------------------------------------------------

// errorText is a call's failure in words.
function errorText(error: unknown): string {
	if (error instanceof NotConnectedError) return "Not connected to the player";
	return error instanceof Error ? error.message : String(error);
}

// adopt shows what a call answered: a State, an Added's state, a Library.
function adopt(value: unknown): void {
	if (!value || typeof value !== "object") return;
	const v = value as Record<string, unknown>;
	if (typeof v.seq === "number" && typeof v.status === "string") showState(value as State);
	else if (v.state && typeof v.state === "object" && typeof v.tracks === "number") showState(v.state as State);
	else if (Array.isArray(v.folders) || (typeof v.version === "number" && typeof v.artists === "number")) showLibrary(value as Library);
}

let order: Promise<unknown> = Promise.resolve();

const app: App = {
	call<T>(work: (p: Player) => Promise<T>): Promise<T | undefined> {
		const next = order.then(async () => {
			if (!player) {
				toast("Not connected to the player yet");
				return undefined;
			}
			try {
				const value = await work(player);
				adopt(value);
				return value;
			} catch (error) {
				toast(errorText(error));
				return undefined;
			}
		});
		order = next;
		return next;
	},
	async ask<T>(work: (p: Player) => Promise<T>): Promise<T | undefined> {
		if (!player) return undefined;
		try {
			return await work(player);
		} catch (error) {
			toast(errorText(error));
			return undefined;
		}
	},
	state: () => state,
	library: () => library,
	show,
};

const playerView = new PlayerView(app);
const queueView = new QueueView(app);
const libraryView = new LibraryView(app);
const settingsView = new SettingsView(app);

// --- the views ------------------------------------------------------------------

const views: View[] = ["queue", "library", "settings"];
let view: View = "queue";

function show(next: View, focus = false): void {
	view = next;
	for (const name of views) {
		const tab = $(`tab-${name}`);
		const selected = name === next;
		tab.setAttribute("aria-selected", String(selected));
		tab.tabIndex = selected ? 0 : -1;
		$(`view-${name}`).hidden = !selected;
		if (selected && focus) tab.focus();
	}
	libraryView.visible(next === "library");
}

for (const name of views) $(`tab-${name}`).addEventListener("click", () => show(name));
// the arrows move along the tabs when one has the focus
$("tab-queue").parentElement?.addEventListener("keydown", (event) => {
	const index = views.indexOf(view);
	let next = -1;
	if (event.key === "ArrowRight") next = (index + 1) % views.length;
	else if (event.key === "ArrowLeft") next = (index - 1 + views.length) % views.length;
	else if (event.key === "Home") next = 0;
	else if (event.key === "End") next = views.length - 1;
	if (next < 0) return;
	event.preventDefault();
	event.stopPropagation();
	show(views[next], true);
});

// --- the states -----------------------------------------------------------------

// showState shows a state newer than the one shown: one given by a call's
// answer and by the states, the newer wins.
function showState(next: State): void {
	if (state && next.seq < state.seq) return;
	state = next;
	playerView.render(next);
	queueView.state(next);
	settingsView.render(next);
	renderError();
	renderLink();
}

function showLibrary(next: Library): void {
	if (library && next.version < library.version) return;
	library = next;
	libraryView.render(next);
	settingsView.library(next);
}

function renderError(): void {
	const text = state?.error ?? "";
	$("error").hidden = !text || text === dismissed;
	$("error-text").textContent = text;
}
$("error-close").addEventListener("click", () => {
	dismissed = state?.error ?? "";
	renderError();
});

// statusText is a connection's status in words: "" once connected.
function statusText(status: Status): { text: string; problem: boolean } {
	switch (status.phase) {
		case "connecting":
			return status.attempt > 1 ? { text: "can't reach the player, retrying…", problem: true } : { text: "connecting…", problem: false };
		case "waiting": {
			const retry = `retrying in ${Math.ceil((status.retryIn ?? 0) / 1000)}s`;
			return { text: status.attempt === 0 ? `lost the player, ${retry}` : `can't reach the player, ${retry}`, problem: true };
		}
		case "closed":
			return { text: "not connected to the player", problem: true };
	}
	return { text: "", problem: false };
}

// daemonStatus shows, while the daemon is away, what the web UI says of
// it: "Starting media-player-musicd…", or why it could not be started,
// with Retry (GET /daemon; the web UI starts it, goframework R115), in
// place of the connection's words; nothing when it has nothing to say.
let daemonProblem = false;
async function daemonStatus(): Promise<void> {
	try {
		const response = await fetch("/daemon", { cache: "no-store" });
		if (!response.ok) return;
		const daemon = (await response.json()) as { text: string; problem: boolean };
		daemonProblem = daemon.problem && link.text !== "";
		if (daemon.text && link.text !== "") link = { text: daemon.text, problem: daemon.problem };
		renderLink();
	} catch {
		// the web UI itself is away: the connection's words stay
	}
}
$("daemon-retry").addEventListener("click", () => {
	daemonProblem = false;
	link = { text: "Starting media-player-musicd…", problem: false };
	renderLink();
	void fetch("/daemon", { method: "POST" })
		.then(() => setTimeout(() => void daemonStatus(), 1000))
		.catch(() => undefined);
});

function renderLink(): void {
	const chip = $("link");
	chip.classList.toggle("problem", link.problem);
	chip.classList.toggle("ok", !link.text);
	const others = state && state.clients > 1 ? ` · ${state.clients} players' UIs` : "";
	chip.textContent = link.text || "connected" + others;
	$("daemon-retry").hidden = !daemonProblem || !link.text;
}

// --- the keys -------------------------------------------------------------------

const commands: Record<string, string> = {
	" ": CommandPlayPause,
	ArrowLeft: CommandSeekBack,
	ArrowRight: CommandSeekForward,
	ArrowUp: CommandVolumeUp,
	ArrowDown: CommandVolumeDown,
	n: CommandNext,
	p: CommandPrevious,
	s: CommandStop,
	m: CommandMute,
	z: CommandShuffle,
	r: CommandRepeat,
	MediaPlayPause: CommandPlayPause,
	MediaTrackNext: CommandNext,
	MediaTrackPrevious: CommandPrevious,
	MediaStop: CommandStop,
};

document.addEventListener("keydown", (event) => {
	if (event.ctrlKey || event.metaKey || event.altKey) return;
	const target = event.target;
	if (isField(target)) {
		if (event.key === "Escape" && target instanceof HTMLElement) target.blur();
		return;
	}
	// a focused button, a tab or a slider takes its own keys
	const own = target instanceof HTMLElement && (target.closest("button, a, [role=tab], input, select, ol.list li") !== null);
	if (event.key === "/") {
		event.preventDefault();
		show("library");
		libraryView.focusSearch();
		return;
	}
	if (["1", "2", "3"].includes(event.key)) {
		show(views[Number(event.key) - 1], true);
		return;
	}
	if (own && (event.key === " " || event.key === "Enter" || event.key.startsWith("Arrow"))) return;
	const command = commands[event.key.length === 1 ? event.key.toLowerCase() : event.key];
	if (!command) return;
	event.preventDefault();
	void app.call((p) => p.Command(command));
});

// --- the connection -------------------------------------------------------------

function start(): void {
	renderLink();
	const url = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/clients`;
	connect({
		url,
		name: "web",
		reconnect: true,
		onConnected: (connection) => {
			player ??= newPlayer(connection);
			// a daemon started again counts its states from the start: the
			// newest it gives now is shown, whatever its number
			state = undefined;
			library = undefined;
			queueView.reset();
		},
		onStatus: (status) => {
			link = statusText(status);
			renderLink();
			if (status.phase !== "connected") void daemonStatus();
		},
		// the newest of each state at each connection, then each newer one,
		// never an older one after a newer one
		states: playerStates({
			[StateEvent]: (next, _revision, deleted) => {
				if (!deleted) showState(next);
			},
			[QueueEvent]: (next, _revision, deleted) => {
				if (deleted) return;
				queueView.render(next);
				if (state) queueView.state(state);
			},
			[NowEvent]: (next, _revision, deleted) => {
				// a Now of a state older than the one shown (from before a seek) is dropped
				if (deleted || (state && next.seq < state.seq)) return;
				playerView.now(next);
			},
			[LibraryEvent]: (next, _revision, deleted) => {
				if (!deleted) showLibrary(next);
			},
		}),
	}).catch((error) => toast(errorText(error))); // refused
}

show("queue");
start();
