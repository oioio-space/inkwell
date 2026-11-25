/**
 * The built-in `activity` extension (plan docs/plans/2026-11-24-obs1-observability.md §2): its
 * manifest declares the metrics, and this editor extension counts them for one document, keyed
 * by its Automerge document id (random, never derived from a name). Per transaction it does one
 * change-set operation and stamps a time; the counting happens at idle (60 s without an edit) or
 * when the editor closes, and the statistics session hands the sums to Go once a minute: typing
 * crosses no bridge (Z-2).
 */
import type { Extension, Transaction } from "@codemirror/state";
import { type EditorView, ViewPlugin, type ViewUpdate } from "@codemirror/view";
import { registry } from "../../extensions/index";
import { statsSession } from "../../obs/index";
import { ActiveTime, IDLE_MS } from "./active";
import { WordTracker } from "./words";

/** The person's own edits: what is typed, deleted or pasted (plan §2). */
const OWN = ["input", "delete", "paste"] as const;
export const isOwn = (tr: Transaction): boolean => OWN.some((e) => tr.isUserEvent(e));

/** What the extension adds to: the page's statistics session. */
export interface Counts {
  add(id: string, v: number, doc?: string): void;
  duration(id: string, ms: number, doc?: string): void;
}

export interface ActivityOptions {
  counts?: Counts;
  now?: () => number;
}

/** The page's session, over the manifests' metrics as the registry merges Go's into them. */
function pageCounts(): Counts {
  const own = registry.metrics.filter((m) => m.from === "manifest");
  return statsSession(own, (go) =>
    registry.withGoMetrics(go).metrics.filter((m) => m.from === "manifest"),
  );
}

/** Counts the person's activity in one document, `doc` being its Automerge document id. */
export function activity(doc: string, { counts, now = Date.now }: ActivityOptions = {}): Extension {
  return ViewPlugin.fromClass(
    class {
      readonly counts = counts ?? pageCounts();
      readonly words: WordTracker;
      readonly active = new ActiveTime();
      readonly opened = now();
      idle: ReturnType<typeof setTimeout> | undefined;

      constructor(readonly view: EditorView) {
        this.words = new WordTracker(view.state.doc);
        this.counts.add("activity.opened", 1, doc);
      }

      update(u: ViewUpdate) {
        if (!u.docChanged) return;
        let own = false;
        let before = u.startState.doc;
        for (const tr of u.transactions) {
          if (!tr.docChanged) continue;
          if (isOwn(tr)) {
            this.words.local(tr.changes, before);
            own = true;
          } else {
            this.words.remote(tr.changes);
            this.counts.add("activity.received", 1, doc);
          }
          before = tr.newDoc;
        }
        if (!own) return;
        this.active.stamp(now());
        this.idle ??= setTimeout(this.check, IDLE_MS);
      }

      /** At idle the window ends; an edit since then puts the check off to a minute after it. */
      readonly check = () => {
        this.idle = undefined;
        const wait = (this.active.last ?? 0) + IDLE_MS - now();
        if (wait > 0) this.idle = setTimeout(this.check, wait);
        else this.end();
      };

      /** Ends the window: the words the person wrote and the time they were active. */
      end() {
        this.counts.add("activity.words", this.words.take(this.view.state.doc), doc);
        const active = this.active.take();
        if (active > 0) this.counts.duration("activity.active", active, doc);
      }

      destroy() {
        clearTimeout(this.idle);
        this.end();
        this.counts.duration("activity.open-time", now() - this.opened, doc);
      }
    },
  );
}
