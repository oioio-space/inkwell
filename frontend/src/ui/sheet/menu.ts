import type { IconName } from "../icons/paths";

export interface MenuItem {
  label: string;
  icon?: IconName;
  shortcut?: string | undefined;
  disabled?: boolean;
  separator?: boolean;
  run?: () => void;
}
