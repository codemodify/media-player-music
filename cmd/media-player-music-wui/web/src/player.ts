// The player: the track playing, where it is, the transport, the levels,
// shuffle and repeat, and the analyser's bands.

import type { App } from "./app.ts";
import { $, latest, spoken, time } from "./dom.ts";
import { CommandMute, CommandNext, CommandPlayPause, CommandPrevious, CommandRepeat, CommandShuffle, CommandStop, RepeatOff, RepeatOne, StatusPlaying, type Now, type State } from "./musicapi.ts";

export class PlayerView {
	private readonly seek = $<HTMLInputElement>("seek");
	private readonly volume = $<HTMLInputElement>("volume");
	private readonly balance = $<HTMLInputElement>("balance");
	private readonly canvas = $<HTMLCanvasElement>("spectrum");
	private seeking = false; // the seek bar is held: the Now does not move it
	private held = new Set<HTMLInputElement>(); // sliders held by the pointer
	private bands: number[] = [];
	private drawing = false;
	private lengthMS = 0;
	private readonly setLevels: (levels: { volume: number; balance: number }) => void;

	constructor(app: App) {
		const command = (name: string) => () => void app.call((p) => p.Command(name));
		$("previous").addEventListener("click", command(CommandPrevious));
		$("play").addEventListener("click", command(CommandPlayPause));
		$("stop").addEventListener("click", command(CommandStop));
		$("next").addEventListener("click", command(CommandNext));
		$("shuffle").addEventListener("click", command(CommandShuffle));
		$("repeat").addEventListener("click", command(CommandRepeat));
		$("mute").addEventListener("click", command(CommandMute));

		this.seek.addEventListener("input", () => {
			this.seeking = true;
			this.showPosition(Number(this.seek.value));
		});
		this.seek.addEventListener("change", () => {
			const at = Number(this.seek.value);
			void app.call((p) => p.Seek(at)).finally(() => (this.seeking = false));
		});

		this.setLevels = latest((levels) => app.call((p) => p.SetLevels(levels)));
		const levels = () => {
			const volume = Number(this.volume.value) / 100;
			const balance = Number(this.balance.value) / 100;
			this.showLevels(volume, balance);
			this.setLevels({ volume, balance });
		};
		for (const slider of [this.volume, this.balance, this.seek]) {
			slider.addEventListener("pointerdown", () => this.held.add(slider));
			slider.addEventListener("pointerup", () => this.held.delete(slider));
			slider.addEventListener("pointercancel", () => this.held.delete(slider));
		}
		this.volume.addEventListener("input", levels);
		this.balance.addEventListener("input", levels);
		// a double click puts the balance back in the middle
		this.balance.addEventListener("dblclick", () => {
			this.balance.value = "0";
			levels();
		});
	}

	// render shows the state.
	render(state: State): void {
		const track = state.track;
		const playing = state.index >= 0 && state.queue.length > 0;
		$("title").textContent = playing ? track.title || fileName(track.path) : "Nothing playing";
		$("artist").textContent = playing ? [track.artist, track.album].filter(Boolean).join(" — ") : "";
		const format = [
			track.rate ? `${(track.rate / 1000).toLocaleString()} kHz` : "",
			track.channels ? (track.channels === 1 ? "mono" : track.channels === 2 ? "stereo" : `${track.channels} channels`) : "",
			track.bitrate ? `${Math.round(track.bitrate / 1000)} kbps` : "",
			playing && state.queue.length > 1 ? `track ${state.index + 1} of ${state.queue.length}` : "",
		].filter(Boolean);
		$("format").textContent = playing ? format.join(" · ") : "";
		document.title = playing && state.status === StatusPlaying ? `▶ ${$("title").textContent} — media-player-music` : "media-player-music";

		const play = $("play");
		const isPlaying = state.status === StatusPlaying;
		play.textContent = isPlaying ? "⏸" : "▶";
		play.setAttribute("aria-label", isPlaying ? "Pause" : "Play");

		const shuffle = $("shuffle");
		shuffle.setAttribute("aria-pressed", String(state.shuffle));
		const repeat = $("repeat");
		repeat.textContent = state.repeat === RepeatOne ? "🔂" : "🔁";
		repeat.setAttribute("aria-label", `Repeat: ${state.repeat}`);
		repeat.setAttribute("aria-pressed", String(state.repeat !== RepeatOff));
		repeat.title = `Repeat: ${state.repeat} (R: off, all, one)`;

		if (!this.held.has(this.volume) && document.activeElement !== this.volume) this.volume.value = String(Math.round(state.volume * 100));
		if (!this.held.has(this.balance) && document.activeElement !== this.balance) this.balance.value = String(Math.round(state.balance * 100));
		this.showLevels(state.volume, state.balance);
		const mute = $("mute");
		mute.textContent = state.volume === 0 ? "🔇" : "🔊";
		mute.setAttribute("aria-label", state.volume === 0 ? "Unmute" : "Mute");

		this.lengthMS = track.lengthMs ?? 0;
		this.place(state.positionMs, this.lengthMS);
		if (state.status !== StatusPlaying) this.draw([]);
	}

	// now shows where the track playing is, and the analyser's bands.
	now(now: Now): void {
		if (now.lengthMs > 0) this.lengthMS = now.lengthMs;
		this.place(now.positionMs, this.lengthMS);
		this.draw(now.bands ?? []);
	}

	private place(positionMS: number, lengthMS: number): void {
		this.seek.max = String(Math.max(0, lengthMS));
		this.seek.disabled = lengthMS <= 0;
		$("length").textContent = time(lengthMS);
		if (this.seeking || this.held.has(this.seek)) return;
		this.seek.value = String(Math.min(positionMS, Math.max(0, lengthMS)));
		this.showPosition(positionMS);
	}

	private showPosition(positionMS: number): void {
		$("position").textContent = time(positionMS);
		this.seek.setAttribute("aria-valuetext", `${spoken(positionMS)} of ${spoken(this.lengthMS)}`);
	}

	private showLevels(volume: number, balance: number): void {
		$("volume-value").textContent = `${Math.round(volume * 100)}%`;
		const b = Math.round(balance * 100);
		$("balance-value").textContent = b === 0 ? "C" : b < 0 ? `L${-b}` : `R${b}`;
		this.balance.setAttribute("aria-valuetext", b === 0 ? "centre" : b < 0 ? `${-b}% left` : `${b}% right`);
		this.volume.setAttribute("aria-valuetext", `${Math.round(volume * 100)}%`);
	}

	// draw draws the bands at the next frame.
	private draw(bands: number[]): void {
		if (bands.length === 0 && this.bands.length === 0) return;
		this.bands = bands;
		if (this.drawing) return;
		this.drawing = true;
		requestAnimationFrame(() => {
			this.drawing = false;
			const context = this.canvas.getContext("2d");
			if (!context) return;
			const { width, height } = this.canvas;
			context.clearRect(0, 0, width, height);
			const count = this.bands.length;
			if (count === 0) return;
			context.fillStyle = getComputedStyle(document.documentElement).getPropertyValue("--bar").trim() || "#0f766e";
			const step = width / count;
			for (let i = 0; i < count; i++) {
				const h = Math.max(2, Math.min(1, this.bands[i]) * height);
				context.fillRect(i * step + 1, height - h, Math.max(1, step - 2), h);
			}
		});
	}
}

// fileName is a path's last name.
export function fileName(path: string): string {
	return path.split(/[\\/]/).pop() ?? path;
}
