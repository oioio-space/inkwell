export type Action =
  | { kind: "next" | "prev" | "first" | "last" | "black" | "white" | "laser" | "quit" }
  | { kind: "goto"; n: number };

/** The part of a keyboard event the keys of a presentation look at. */
export interface KeyLike {
  key: string;
  ctrlKey?: boolean;
  altKey?: boolean;
  metaKey?: boolean;
}

const MAX_DIGITS = 4;

/**
 * The keys of a presentation: arrows, space, PageUp and PageDown, Home and End, B and W for a
 * black or white screen, L for the laser, Esc to leave; a number then Enter jumps to that slide.
 * It keeps the digits typed so far, which the presenter shows while they are typed.
 */
export class KeyReader {
  /** The digits typed, not yet confirmed with Enter. */
  buffer = "";

  /** The action of a key, or undefined when it means nothing here (the page then keeps the key). */
  read(e: KeyLike): Action | undefined {
    if (e.ctrlKey || e.altKey || e.metaKey) return undefined;
    if (/^[0-9]$/.test(e.key)) {
      if (this.buffer.length < MAX_DIGITS) this.buffer += e.key;
      return undefined;
    }
    const typed = this.buffer;
    switch (e.key) {
      case "Enter":
        this.buffer = "";
        return typed ? { kind: "goto", n: Number(typed) } : { kind: "next" };
      case "Escape":
        this.buffer = "";
        return typed ? undefined : { kind: "quit" };
      case "Backspace":
        if (typed) {
          this.buffer = typed.slice(0, -1);
          return undefined;
        }
        return { kind: "prev" };
    }
    this.buffer = "";
    switch (e.key) {
      case " ":
      case "ArrowRight":
      case "ArrowDown":
      case "PageDown":
        return { kind: "next" };
      case "ArrowLeft":
      case "ArrowUp":
      case "PageUp":
        return { kind: "prev" };
      case "Home":
        return { kind: "first" };
      case "End":
        return { kind: "last" };
      case "b":
      case "B":
        return { kind: "black" };
      case "w":
      case "W":
        return { kind: "white" };
      case "l":
      case "L":
        return { kind: "laser" };
    }
    return undefined;
  }
}
