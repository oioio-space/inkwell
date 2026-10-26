/** A short message at the bottom of the window (an AdwToast) that goes away by itself. */
class Notice {
  text = $state("");
  /** "alert" interrupts a screen reader: for what failed. */
  role = $state<"alert" | "status">("status");
  #timer: ReturnType<typeof setTimeout> | undefined;

  show(text: string, role: "alert" | "status" = "status", ms = 7000): void {
    clearTimeout(this.#timer);
    this.text = text;
    this.role = role;
    this.#timer = setTimeout(() => this.hide(), ms);
  }

  hide(): void {
    clearTimeout(this.#timer);
    this.text = "";
  }
}

export const notice = new Notice();
