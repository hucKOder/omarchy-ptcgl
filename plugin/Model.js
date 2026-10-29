// Shared by QML and the Node-based contract tests. Keep this file free of I/O.
var minimumVersion = "0.4.0";
var setupChecks = ["Proton", "prefix", "Visual C++ runtime", "game"];

function loading() {
  return { state: "loading", version: "", detail: "Checking the installed launcher…" };
}

function versionResult(output, exitCode) {
  if (exitCode === 44)
    return { state: "missing", version: "", detail: "Install the launcher, then prepare the game on first launch." };
  var match = String(output).trim().match(/^ptcgl-omarchy (\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$/);
  if (exitCode !== 0 || !match)
    return { state: "unavailable", version: "", detail: "The launcher could not report its version. Retry, or reinstall the launcher." };
  var version = match.slice(1, 4).join(".") + (match[4] || "");
  var minimum = minimumVersion.split(".");
  var comparison = 0;
  for (var i = 0; i < minimum.length; i++) {
    comparison = Number(match[i + 1]) - Number(minimum[i]);
    if (comparison !== 0) break;
  }
  var supported = comparison > 0 || (comparison === 0 && !match[4]);
  return {
    state: supported ? "compatible" : "outdated",
    version: version,
    detail: supported ? "Checking game setup and desktop integration…" : "Launcher " + minimumVersion + " or newer is required."
  };
}

function diagnosticResult(output, exitCode) {
  var unavailable = { state: "unavailable", detail: "Diagnostics could not be read. Open diagnostics for the error, then retry." };
  if (exitCode !== 0 && exitCode !== 1) return unavailable;
  var checks;
  try { checks = JSON.parse(output); } catch (error) { return unavailable; }
  if (!Array.isArray(checks) || checks.length === 0) return unavailable;
  var seen = Object.create(null);
  var failed = [];
  for (var i = 0; i < checks.length; i++) {
    var check = checks[i];
    if (!check || typeof check.name !== "string" || !check.name ||
        typeof check.ok !== "boolean" || typeof check.detail !== "string" ||
        (check.fix !== undefined && typeof check.fix !== "string") ||
        Object.prototype.hasOwnProperty.call(seen, check.name)) return unavailable;
    seen[check.name] = true;
    if (!check.ok) failed.push(check);
  }
  for (var j = 0; j < setupChecks.length; j++) {
    if (!Object.prototype.hasOwnProperty.call(seen, setupChecks[j])) return unavailable;
  }
  // CLI exit status and report must agree, including the successful-check case.
  if ((failed.length === 0) !== (exitCode === 0)) return unavailable;
  if (failed.length === 0)
    return { state: "ready", detail: "Setup checks passed. Play launches the game or focuses its existing window." };
  var setupOnly = failed.every(function(check) { return setupChecks.indexOf(check.name) !== -1; });
  return {
    state: setupOnly ? "setup-needed" : "attention",
    detail: setupOnly ? "First launch will prepare Proton and install missing game files." :
      failed[0].name + ": " + failed[0].detail.slice(0, 240)
  };
}

function title(info, diagnostics) {
  if (info.state === "loading") return "Checking launcher…";
  if (info.state === "missing") return "Launcher not installed";
  if (info.state === "outdated") return "Launcher upgrade needed";
  if (info.state !== "compatible") return "Launcher unavailable";
  if (diagnostics.state === "loading") return "Checking setup…";
  var titles = {
    "running": "Game running", "starting": "Starting game…", "setup": "Installing game…",
    "update": "Updating installer…", "repair": "Repairing game…", "check": "Checking installer…",
    "runtime-update": "Preparing runtime…", "maintenance": "Maintenance in progress…",
    "repair-needed": "Game repair needed", "outdated": "Launcher upgrade needed"
  };
  if (titles[diagnostics.state]) return titles[diagnostics.state];
  if (diagnostics.state === "ready") return "Ready to play";
  if (diagnostics.state === "setup-needed") return "Game setup needed";
  if (diagnostics.state === "attention") return "Needs attention";
  return "Status unavailable";
}

function statusResult(output, exitCode) {
  var unavailable = { state: "unavailable", detail: "Live status unavailable. Open diagnostics; maintenance is disabled until activity can be established." };
  if (exitCode === 2) return { state: "outdated", detail: "Upgrade the launcher to a version with live status schema 1." };
  if (exitCode !== 0) return unavailable;
  var value;
  try { value = JSON.parse(output); } catch (error) { return unavailable; }
  if (!value || value.schema_version !== 1 || typeof value.launcher_version !== "string" ||
      ["ready", "setup-needed", "repair-needed", "unavailable"].indexOf(value.readiness) < 0 ||
      ["idle", "starting", "running", "play", "setup", "update", "repair", "check", "runtime-update", "maintenance", "unavailable"].indexOf(value.activity) < 0 ||
      typeof value.phase !== "string" || typeof value.message !== "string" || typeof value.runtime !== "string" ||
      typeof value.custom_runtime !== "boolean" || !value.installer ||
      ["not-checked", "unchanged-at-check", "available-at-check"].indexOf(value.installer.freshness) < 0 || !value.capabilities) return unavailable;
  if ((value.installer.last_applied !== undefined && typeof value.installer.last_applied !== "string") ||
      (value.installer.checked_at !== undefined && typeof value.installer.checked_at !== "string")) return unavailable;
  var caps = value.capabilities;
  for (var i = 0; i < 4; i++) {
    if (typeof caps[["play", "maintain", "diagnostics", "folders"][i]] !== "boolean") return unavailable;
  }
  // Contradictory snapshots must never enable maintenance or a duplicate launch.
  if (caps.maintain && value.activity !== "idle") return unavailable;
  if (caps.play && ["idle", "running"].indexOf(value.activity) < 0) return unavailable;
  if (caps.play && value.activity === "idle" && ["repair-needed", "unavailable"].indexOf(value.readiness) >= 0) return unavailable;
  var state = value.activity === "idle" ? value.readiness : value.activity;
  if (state === "play") state = "starting";
  var detail = value.message;
  var progress = value.progress;
  if (progress) {
    if (typeof progress.downloaded !== "number" || !isFinite(progress.downloaded) || progress.downloaded < 0 ||
        typeof progress.total !== "number" || !isFinite(progress.total) || progress.total < -1 ||
        typeof progress.bytes_per_second !== "number" || !isFinite(progress.bytes_per_second) || progress.bytes_per_second < 0 ||
        typeof progress.cancellable !== "boolean" || typeof progress.phase !== "string" || typeof progress.message !== "string") return unavailable;
    if (progress.phase === "downloading") {
      detail = "Downloaded " + (progress.downloaded / 1048576).toFixed(1) + " MiB";
      if (progress.total > 0) detail += " / " + (progress.total / 1048576).toFixed(1) + " MiB · " + Math.min(100, Math.floor(progress.downloaded / progress.total * 100)) + "%";
      else detail += " · total size unknown";
      if (progress.bytes_per_second > 0) detail += " · " + (progress.bytes_per_second / 1048576).toFixed(1) + " MiB/s";
    }
  }
  if (!caps.maintain && value.activity !== "idle") detail += " Maintenance requires the game and other operations to be closed.";
  return { state: state, detail: detail, capabilities: caps, snapshot: value };
}

function updateDetail(diagnostics) {
  var value = diagnostics.snapshot;
  if (!value) return "Versions and installer state unavailable.";
  var text = "Launcher: " + value.launcher_version + " (online freshness not checked)\nRuntime: " + value.runtime;
  text += "\nInstaller last applied: " + (value.installer.last_applied || "not recorded");
  text += "\nInstaller: " + (value.installer.freshness === "not-checked" ? "not checked" : value.installer.freshness.replace(/-/g, " "));
  if (value.installer.checked_at) text += " · " + value.installer.checked_at;
  return text + "\nGame patches are checked in-game.";
}

function actions(info, diagnostics, refreshing, expanded) {
  expanded = expanded || {};
  var compatible = info.state === "compatible";
  var caps = diagnostics.capabilities || {};
  var maintain = compatible && caps.maintain === true && !refreshing;
  var primary = { id: "install", section: "", text: "Install launcher", enabled: !refreshing, primary: true };
  if (info.state === "loading") {
    primary = { id: "refresh", section: "", text: "Checking launcher…", enabled: false, primary: true };
  } else if (info.state === "outdated" || info.state === "unavailable") {
    primary.text = info.state === "outdated" ? "Upgrade launcher" : "Reinstall launcher";
  } else if (compatible && diagnostics.state === "outdated") {
    primary.text = "Upgrade launcher";
  } else if (compatible) {
    var play = diagnostics.state === "ready" || diagnostics.state === "setup-needed";
    primary = {
      id: play ? "play" : "doctor", section: "",
      text: diagnostics.state === "setup-needed" ? "Set up & play" : (play ? "Play" : "Open diagnostics"),
      enabled: !refreshing, primary: true
    };
    if (play) primary.enabled = caps.play === true && !refreshing;
    if (diagnostics.state === "running") primary = { id: "play", text: "Focus game", primary: true, enabled: caps.play === true && !refreshing };
    if (diagnostics.state === "repair-needed") primary = { id: "repair", text: "Repair game", primary: true, enabled: maintain };
    if (["starting", "setup", "update", "repair", "check", "runtime-update", "maintenance", "loading"].indexOf(diagnostics.state) >= 0)
      primary = { id: "play", text: title(info, diagnostics), primary: true, enabled: false };
  }
  var result = [primary];
  result.push({ id: "toggle:maintenance", text: (expanded.maintenance ? "▾ " : "▸ ") + "Maintenance", enabled: true, group: true });
  if (expanded.maintenance) result = result.concat([
    { id: "updates", text: "Versions & updates…", enabled: compatible },
    { id: "check", text: "Check installer updates", enabled: maintain },
    { id: "update", text: "Update game installer", enabled: maintain },
    { id: "repair", text: "Repair game…", enabled: maintain },
    { id: "doctor", text: "Diagnostics & recovery…", enabled: compatible }
  ]);
  result.push({ id: "toggle:folders", text: (expanded.folders ? "▾ " : "▸ ") + "Folders", enabled: true, group: true });
  if (expanded.folders) result = result.concat([
    { id: "game", text: "Open game folder", enabled: compatible },
    { id: "config", text: "Open configuration folder", enabled: compatible },
    { id: "cache", text: "Open download cache", enabled: compatible }
  ]);
  result.push({ id: "toggle:advanced", text: (expanded.advanced ? "▾ " : "▸ ") + "Advanced", enabled: true, group: true });
  if (expanded.advanced) {
    if (compatible) result.push({ id: "install", text: "Install / upgrade launcher…", enabled: maintain || diagnostics.state === "outdated" });
    result.push({ id: "runtime", text: "Select bundled runtime…", enabled: !!(maintain && diagnostics.snapshot && !diagnostics.snapshot.custom_runtime) });
    result.push({ id: "refresh", text: "Refresh status", enabled: !refreshing });
  }
  return result;
}

function actionCommand(id, launcher, helper) {
  if (id === "install") return ["omarchy", "launch", "terminal", "bash", helper];
  if (id === "play") return [launcher, "app"];
  if (id === "check") return [launcher, "update", "--check", "--gui"];
  if (id === "update") return [launcher, "update", "--gui"];
  if (id === "repair") return [launcher, "fix", "--gui", "repair"];
  if (id === "runtime") return [launcher, "fix", "--gui", "runtime-update"];
  if (id === "updates") return [launcher, "updates", "--gui"];
  if (id === "doctor") return [launcher, "doctor", "--gui"];
  if (id === "game" || id === "config" || id === "cache") return [launcher, "open", "--gui", id];
  return [];
}

function localPath(url) {
  var value = String(url);
  if (value.indexOf("file:///") !== 0) throw new Error("Expected a local plugin file");
  return decodeURIComponent(value.slice(7));
}

function nextEnabled(items, index, direction) {
  if (index < 0 || index >= items.length) index = direction > 0 ? -1 : 0;
  for (var i = 0; i < items.length; i++) {
    index = (index + direction + items.length) % items.length;
    if (items[index].enabled) return index;
  }
  return -1;
}
