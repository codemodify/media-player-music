// The settings: the equaliser, ReplayGain, and the library's folders (add,
// remove, read again) with how the reading goes.

import type { App } from "./app.ts";
import { $, button, el, latest, plural, time, toast } from "./dom.ts";
import { Bands, EqualizerRange, Presets, ReplayGainOff, type Equalizer, type Library, type ReplayGain, type State } from "./musicapi.ts";

export class SettingsView {
	private readonly on = $<HTMLInputElement>("eq-on");
	private readonly auto = $<HTMLInputElement>("eq-auto");
	private readonly preset = $<HTMLSelectElement>("eq-preset");
	private readonly preamp: HTMLInputElement;
	private readonly gains: HTMLInputElement[] = [];
	private readonly outputs: HTMLOutputElement[] = [];
	private readonly rgPreamp = $<HTMLInputElement>("rg-preamp");
	private readonly held = new Set<HTMLInputElement>(); // sliders being moved
	private equalizer: Equalizer | undefined;
	private gain: ReplayGain | undefined;
	private readonly sendEqualizer: (equalizer: Equalizer) => void;
	private readonly sendGain: (gain: ReplayGain) => void;
	private folders = "";

	constructor(private readonly app: App) {
		this.sendEqualizer = latest((equalizer) => app.call((p) => p.SetEqualizer(equalizer)));
		this.sendGain = latest((gain) => app.call((p) => p.SetReplayGain(gain)));

		// the equaliser: the preamp, then the bands, each a vertical slider
		const eq = $("eq");
		const slider = (label: string, spoken: string) => {
			const input = el("input", {
				type: "range",
				min: -EqualizerRange,
				max: EqualizerRange,
				step: 0.5,
				value: 0,
				"aria-label": spoken,
				"aria-orientation": "vertical",
			});
			const output = el("output", {}, "0");
			input.addEventListener("pointerdown", () => this.held.add(input));
			input.addEventListener("pointerup", () => this.held.delete(input));
			input.addEventListener("pointercancel", () => this.held.delete(input));
			input.addEventListener("dblclick", () => {
				input.value = "0";
				input.dispatchEvent(new Event("input"));
			});
			eq.append(el("div", { className: label === "Pre" ? "band preamp" : "band" }, output, input, el("span", {}, label)));
			this.outputs.push(output);
			return input;
		};
		this.preamp = slider("Pre", "Equaliser preamp, dB");
		this.preamp.addEventListener("input", () => this.changeEqualizer((e) => (e.preamp = Number(this.preamp.value))));
		Bands.forEach((band, index) => {
			const input = slider(band, `${band}${band.endsWith("k") ? "Hz" : " Hz"} band, dB`);
			input.addEventListener("input", () =>
				this.changeEqualizer((e) => {
					e.gains = this.gains.map((g) => Number(g.value));
					e.gains[index] = Number(input.value);
					e.preset = "Custom";
				}),
			);
			this.gains.push(input);
		});
		this.preset.replaceChildren(
			...Presets.map((name) => el("option", { value: name }, name)),
			el("option", { value: "Custom", disabled: true }, "Custom"),
		);
		this.preset.addEventListener("change", () =>
			this.changeEqualizer((e) => {
				e.preset = this.preset.value;
				e.gains = []; // the preset's
			}),
		);
		this.on.addEventListener("change", () => this.changeEqualizer((e) => (e.on = this.on.checked)));
		this.auto.addEventListener("change", () => this.changeEqualizer((e) => (e.auto = this.auto.checked)));

		// ReplayGain
		for (const choice of $("rg-mode").querySelectorAll<HTMLButtonElement>("button")) {
			choice.addEventListener("click", () => this.changeGain((g) => (g.mode = choice.dataset.mode ?? ReplayGainOff)));
		}
		this.rgPreamp.addEventListener("pointerdown", () => this.held.add(this.rgPreamp));
		this.rgPreamp.addEventListener("pointerup", () => this.held.delete(this.rgPreamp));
		this.rgPreamp.addEventListener("input", () => {
			$("rg-preamp-value").textContent = dB(Number(this.rgPreamp.value));
			this.changeGain((g) => (g.preamp = Number(this.rgPreamp.value)));
		});

		// the library's folders
		const path = $<HTMLInputElement>("folder-path");
		const add = async () => {
			const folder = path.value.trim();
			if (!folder) {
				path.focus();
				return;
			}
			if ((await app.call((p) => p.AddFolder(folder))) !== undefined) {
				path.value = "";
				toast(`Reading ${folder}…`);
			}
		};
		$("add-folder").addEventListener("click", () => void add());
		path.addEventListener("keydown", (event) => {
			if (event.key === "Enter") void add();
		});
		$("rescan").addEventListener("click", () => void app.call((p) => p.Rescan()));
	}

	private changeEqualizer(change: (equalizer: Equalizer) => void): void {
		if (!this.equalizer) return;
		const next: Equalizer = { ...this.equalizer, gains: [...(this.equalizer.gains ?? [])] };
		change(next);
		this.equalizer = next;
		this.showEqualizer(next);
		this.sendEqualizer(next);
	}

	private changeGain(change: (gain: ReplayGain) => void): void {
		if (!this.gain) return;
		const next = { ...this.gain };
		change(next);
		this.gain = next;
		this.showGain(next);
		this.sendGain(next);
	}

	// render shows the sound's settings.
	render(state: State): void {
		this.equalizer = state.equalizer;
		this.gain = state.replayGain;
		this.showEqualizer(state.equalizer);
		this.showGain(state.replayGain);
		$("formats").textContent = state.formats ? `Plays ${state.formats}.` : "";
	}

	private showEqualizer(e: Equalizer): void {
		this.on.checked = e.on;
		this.auto.checked = !!e.auto;
		this.preset.value = e.preset && (Presets.includes(e.preset) || e.preset === "Custom") ? e.preset : "Custom";
		$("eq").classList.toggle("off", !e.on);
		const set = (input: HTMLInputElement, output: HTMLOutputElement, value: number) => {
			if (!this.held.has(input)) input.value = String(value);
			const shown = this.held.has(input) ? Number(input.value) : value;
			output.textContent = shown > 0 ? `+${shown}` : String(shown);
			input.setAttribute("aria-valuetext", dB(shown));
		};
		set(this.preamp, this.outputs[0], e.preamp);
		this.gains.forEach((input, index) => set(input, this.outputs[index + 1], e.gains?.[index] ?? 0));
	}

	private showGain(g: ReplayGain): void {
		for (const choice of $("rg-mode").querySelectorAll<HTMLButtonElement>("button")) {
			choice.setAttribute("aria-pressed", String(choice.dataset.mode === g.mode));
		}
		if (!this.held.has(this.rgPreamp)) this.rgPreamp.value = String(g.preamp);
		$("rg-preamp-value").textContent = dB(Number(this.rgPreamp.value));
		this.rgPreamp.setAttribute("aria-valuetext", dB(Number(this.rgPreamp.value)));
		this.rgPreamp.disabled = g.mode === ReplayGainOff;
	}

	// library shows the library in short: how much, its folders, how the
	// reading goes, what went wrong.
	library(library: Library): void {
		const stats = $("library-stats");
		stats.replaceChildren(
			...(
				[
					["Tracks", library.tracks.toLocaleString()],
					["Artists", library.artists.toLocaleString()],
					["Albums", library.albums.toLocaleString()],
					["Genres", library.genres.toLocaleString()],
					["Plays for", time(library.lengthMs)],
				] as const
			).flatMap(([name, value]) => [el("dt", {}, name), el("dd", {}, value)]),
		);
		const scan = $("scan");
		scan.hidden = !library.scanning;
		scan.textContent = library.scanning ? `Reading the folders… ${plural(library.scanned ?? 0, "file")} so far` : "";
		$<HTMLButtonElement>("rescan").disabled = !!library.scanning || (library.folders ?? []).length === 0;
		const error = $("library-error");
		error.hidden = !library.error;
		error.textContent = library.error ?? "";

		const folders = library.folders ?? [];
		const key = folders.join("\n");
		if (key === this.folders) return;
		this.folders = key;
		const list = $("folders");
		list.replaceChildren(
			...(folders.length === 0
				? [el("li", {}, el("span", {}, "No folders yet: add one below."))]
				: folders.map((folder) =>
						el(
							"li",
							{},
							el("span", {}, folder),
							button("Remove", `Remove ${folder} from the library`, () => void this.app.call((p) => p.RemoveFolder(folder)), "btn small danger"),
						),
					)),
		);
	}
}

function dB(value: number): string {
	return `${value > 0 ? "+" : ""}${value} dB`;
}
