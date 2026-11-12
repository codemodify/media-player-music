// The page's small helpers: finding and making elements, times in words,
// the toast.

export const $ = <T extends HTMLElement = HTMLElement>(id: string): T => document.getElementById(id) as T;

type Child = Node | string | null | undefined | false;

// el makes an element with its properties (attributes for names with a
// dash or aria-…, properties otherwise) and its children.
export function el<K extends keyof HTMLElementTagNameMap>(
	tag: K,
	props: Record<string, string | number | boolean | undefined> = {},
	...children: Child[]
): HTMLElementTagNameMap[K] {
	const element = document.createElement(tag);
	for (const [name, value] of Object.entries(props)) {
		if (value === undefined || value === false) continue;
		if (name.includes("-") || name === "role") element.setAttribute(name, String(value));
		else (element as unknown as Record<string, unknown>)[name] = value;
	}
	for (const child of children) {
		if (child) element.append(child);
	}
	return element;
}

// button makes a button that does action when clicked.
export function button(
	text: string,
	label: string,
	action: () => void,
	className = "icon",
): HTMLButtonElement {
	const element = el("button", { type: "button", className, "aria-label": label, title: label }, text);
	element.addEventListener("click", (event) => {
		event.stopPropagation();
		action();
	});
	return element;
}

// time is a length in milliseconds as a player shows it: "3:07",
// "1:02:09".
export function time(ms: number): string {
	const total = Math.max(0, Math.floor(ms / 1000));
	const seconds = total % 60;
	const minutes = Math.floor(total / 60) % 60;
	const hours = Math.floor(total / 3600);
	const ss = String(seconds).padStart(2, "0");
	return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${ss}` : `${minutes}:${ss}`;
}

// spoken is a length in words for a screen reader: "3 minutes 7 seconds".
export function spoken(ms: number): string {
	const total = Math.max(0, Math.floor(ms / 1000));
	const parts: string[] = [];
	const hours = Math.floor(total / 3600);
	const minutes = Math.floor(total / 60) % 60;
	const seconds = total % 60;
	if (hours) parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
	if (minutes) parts.push(`${minutes} minute${minutes === 1 ? "" : "s"}`);
	if (seconds || parts.length === 0) parts.push(`${seconds} second${seconds === 1 ? "" : "s"}`);
	return parts.join(" ");
}

// plural is a count and its noun: "1 track", "12 tracks".
export function plural(count: number, noun: string): string {
	return `${count.toLocaleString()} ${noun}${count === 1 ? "" : "s"}`;
}

// isField tells whether typing goes to target: an input, a select, a text
// area.
export function isField(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;
	if (target instanceof HTMLInputElement) return !["checkbox", "radio", "button", "submit", "range"].includes(target.type);
	return target instanceof HTMLSelectElement || target instanceof HTMLTextAreaElement || target.isContentEditable;
}

let toastTimer = 0;

// toast says text for a moment.
export function toast(text: string): void {
	const element = $("toast");
	element.textContent = text;
	element.classList.add("shown");
	clearTimeout(toastTimer);
	toastTimer = window.setTimeout(() => element.classList.remove("shown"), 3500);
}

// latest calls send with the newest value given, one call at a time: a
// slider dragged sends where it is, not every step it passed.
export function latest<T>(send: (value: T) => Promise<unknown>): (value: T) => void {
	let pending: { value: T } | undefined;
	let busy = false;
	const run = async (): Promise<void> => {
		busy = true;
		while (pending) {
			const { value } = pending;
			pending = undefined;
			try {
				await send(value);
			} catch {
				// said by send's caller
			}
		}
		busy = false;
	};
	return (value: T) => {
		pending = { value };
		if (!busy) void run();
	};
}
