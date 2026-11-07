import { call, event } from "mygo-runtime";
import type {
  NavSearchResult,
  NavSnapshot,
  RecentWorkspace,
} from "../types";

export const WorkspaceNav = {
  navSnapshot(): Promise<NavSnapshot> {
    return call("WorkspaceHost.NavSnapshot");
  },
  setFilter(query: string): Promise<NavSearchResult> {
    return call("WorkspaceHost.SetFilter", query);
  },
  toggleFolder(rel: string): Promise<void> {
    return call("WorkspaceHost.ToggleFolder", rel);
  },
  revealPath(path: string): Promise<void> {
    return call("WorkspaceHost.RevealPath", path);
  },
  pickFolder(): Promise<string> {
    return call("WorkspaceHost.PickFolder");
  },
  recentWorkspaces(): Promise<RecentWorkspace[]> {
    return call("WorkspaceHost.RecentWorkspaces");
  },
} as const;

export const navEvents = {
  navChanged: event<NavSnapshot>("navChanged"),
  navSearchReady: event<NavSearchResult>("navSearchReady"),
} as const;
