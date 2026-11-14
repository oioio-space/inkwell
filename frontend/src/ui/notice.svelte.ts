/** A short message at the bottom of the window (an AdwToast) that goes away by itself. */
class Notice {
  text = $state("");
  /** "alert" interrupts a screen reader: for what failed. */
  role = $state<"alert" | "status">("status");
  #timer: ReturnType<typeof setTimeout> | undefined;
  #queue: { text: string; role: "alert" | "status"; ms: number }[] = [];

  show(text: string, role: "alert" | "status" = "status", ms = 7000): void {
    clearTimeout(this.#timer);
    this.text = text;
    this.role = role;
    this.#timer = setTimeout(() => this.hide(), ms);
  }

  /** Shows text once the toasts already up or queued have gone (AdwToastOverlay: one at a time). */
  queue(text: string, role: "alert" | "status" = "status", ms = 7000): void {
    if (this.text) this.#queue.push({ text, role, ms });
    else this.show(text, role, ms);
  }

  hide(): void {
    clearTimeout(this.#timer);
    this.text = "";
    const next = this.#queue.shift();
    if (next) this.show(next.text, next.role, next.ms);
  }
}

export const notice = new Notice();
